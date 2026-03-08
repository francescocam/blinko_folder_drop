package processor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"blinko-folder-drop/internal/blinko"
)

func TestMoveFile(t *testing.T) {
	d := t.TempDir()
	src := filepath.Join(d, "a.txt")
	dst := filepath.Join(d, "sub", "a.txt")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := moveFile(src, dst); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("expected destination file: %v", err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("expected source removed, err=%v", err)
	}
}

type fakeClient struct {
	uploadResp   blinko.UploadResponse
	uploadErr    error
	upsertErr    error
	uploadCalls  int
	upsertCalls  int
	lastUpsert   blinko.NoteUpsertRequest
	lastUploadTo string
}

func (f *fakeClient) UploadFile(_ context.Context, path string) (blinko.UploadResponse, error) {
	f.uploadCalls++
	f.lastUploadTo = path
	return f.uploadResp, f.uploadErr
}

func (f *fakeClient) UpsertNote(_ context.Context, req blinko.NoteUpsertRequest) error {
	f.upsertCalls++
	f.lastUpsert = req
	return f.upsertErr
}

func TestProcessMarkdownCreatesTextOnlyNote(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "a.md")
	if err := os.WriteFile(p, []byte("# title"), 0o644); err != nil {
		t.Fatal(err)
	}

	fc := &fakeClient{}
	proc := New(fc, Config{DeleteOnOK: true, FailedDir: filepath.Join(d, "failed")})
	if err := proc.Process(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	if fc.uploadCalls != 0 {
		t.Fatalf("expected no upload for markdown, got %d", fc.uploadCalls)
	}
	if fc.upsertCalls != 1 {
		t.Fatalf("expected 1 upsert, got %d", fc.upsertCalls)
	}
	if fc.lastUpsert.Content != "# title" || len(fc.lastUpsert.Attachments) != 0 || fc.lastUpsert.Type != 1 {
		t.Fatalf("unexpected upsert payload: %+v", fc.lastUpsert)
	}
}

func TestProcessBinaryUploadsAndBuildsAttachmentNote(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "a.bin")
	if err := os.WriteFile(p, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}

	fc := &fakeClient{
		uploadResp: blinko.UploadResponse{Path: "/api/file/abc", Name: "a.bin", Size: 3, Type: "application/octet-stream"},
	}
	proc := New(fc, Config{DeleteOnOK: true, FailedDir: filepath.Join(d, "failed")})
	if err := proc.Process(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	if fc.uploadCalls != 1 || fc.upsertCalls != 1 {
		t.Fatalf("unexpected calls upload=%d upsert=%d", fc.uploadCalls, fc.upsertCalls)
	}
	if len(fc.lastUpsert.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(fc.lastUpsert.Attachments))
	}
	if fc.lastUpsert.Type != 1 {
		t.Fatalf("unexpected note type: %+v", fc.lastUpsert)
	}
	if fc.lastUpsert.Attachments[0].Path != "/api/file/abc" {
		t.Fatalf("unexpected attachment path: %+v", fc.lastUpsert.Attachments[0])
	}
}
