package processor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"blinko-folder-drop/internal/blinko"
)

type Config struct {
	DeleteOnOK bool
	ArchiveDir string
	FailedDir  string
}

type BlinkoClient interface {
	UploadFile(ctx context.Context, path string) (blinko.UploadResponse, error)
	UpsertNote(ctx context.Context, reqBody blinko.NoteUpsertRequest) error
}

type Processor struct {
	client BlinkoClient
	cfg    Config
}

type FailureRecord struct {
	File      string    `json:"file"`
	Error     string    `json:"error"`
	At        time.Time `json:"at"`
	Attempts  int       `json:"attempts"`
	Permanent bool      `json:"permanent"`
}

func New(client BlinkoClient, cfg Config) *Processor { return &Processor{client: client, cfg: cfg} }

func (p *Processor) Process(ctx context.Context, filePath string) error {
	const defaultNoteType = 1
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == ".md" || ext == ".markdown" {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		return p.client.UpsertNote(ctx, blinko.NoteUpsertRequest{
			Content:     string(content),
			Type:        defaultNoteType,
			Attachments: []blinko.Attachment{},
		})
	}

	u, err := p.client.UploadFile(ctx, filePath)
	if err != nil {
		return err
	}
	name := filepath.Base(filePath)
	content := fmt.Sprintf("# %s\n\nImported by folder-drop service.", name)
	return p.client.UpsertNote(ctx, blinko.NoteUpsertRequest{
		Content: content,
		Type:    defaultNoteType,
		Attachments: []blinko.Attachment{{
			Name: u.Name,
			Path: u.Path,
			Size: u.Size,
			Type: u.Type,
		}},
	})
}

func (p *Processor) FinalizeSuccess(path string) (bool, error) {
	if p.cfg.DeleteOnOK {
		return true, os.Remove(path)
	}
	base := filepath.Base(path)
	dst := filepath.Join(p.cfg.ArchiveDir, base)
	return false, moveFile(path, dst)
}

func (p *Processor) Quarantine(path string, rec FailureRecord) error {
	if err := os.MkdirAll(p.cfg.FailedDir, 0o755); err != nil {
		return err
	}
	base := filepath.Base(path)
	dst := filepath.Join(p.cfg.FailedDir, base)
	if err := moveFile(path, dst); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}

	rec.File = dst
	b, _ := json.MarshalIndent(rec, "", "  ")
	return os.WriteFile(dst+".error.json", b, 0o644)
}

func moveFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := out.ReadFrom(in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Remove(src)
}
