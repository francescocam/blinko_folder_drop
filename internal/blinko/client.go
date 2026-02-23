package blinko

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

type UploadResponse struct {
	Path string      `json:"path"`
	Name string      `json:"name"`
	Size interface{} `json:"size"`
	Type string      `json:"type"`
}

type Attachment struct {
	Name string      `json:"name"`
	Path string      `json:"path"`
	Size interface{} `json:"size"`
	Type string      `json:"type"`
}

type NoteUpsertRequest struct {
	Content     string       `json:"content"`
	Type        int          `json:"type"`
	Attachments []Attachment `json:"attachments"`
}

type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("http status %d: %s", e.StatusCode, e.Body)
}

func New(baseURL, token string, hc *http.Client) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), token: token, http: hc}
}

func (c *Client) UploadFile(ctx context.Context, path string) (UploadResponse, error) {
	var out UploadResponse

	f, err := os.Open(path)
	if err != nil {
		return out, err
	}
	defer f.Close()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return out, err
	}
	if _, err := io.Copy(part, f); err != nil {
		return out, err
	}
	if err := mw.Close(); err != nil {
		return out, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/file/upload", &buf)
	if err != nil {
		return out, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := c.http.Do(req)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return out, &HTTPError{StatusCode: resp.StatusCode, Body: string(body)}
	}

	if err := json.Unmarshal(body, &out); err != nil {
		return out, fmt.Errorf("parse upload response: %w", err)
	}
	if out.Path == "" {
		return out, fmt.Errorf("upload response missing path")
	}
	return out, nil
}

func (c *Client) UpsertNote(ctx context.Context, reqBody NoteUpsertRequest) error {
	b, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/note/upsert", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return &HTTPError{StatusCode: resp.StatusCode, Body: string(body)}
	}
	return nil
}
