package queue

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"blinko-folder-drop/internal/blinko"
	"blinko-folder-drop/internal/metrics"
	"blinko-folder-drop/internal/processor"
	"blinko-folder-drop/internal/store"
)

func TestMarkdownCreatesNoteAndDeletesFile(t *testing.T) {
	var upserts atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/note/upsert" {
			upserts.Add(1)
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	dir := t.TempDir()
	f := filepath.Join(dir, "note.md")
	mustWrite(t, f, "# hello")

	failedDir := filepath.Join(dir, "failed")
	q, cancel := newTestQueue(t, srv.URL, failedDir, true, "", 1, 2, 10*time.Millisecond)
	defer cancel()
	q.EnqueuePath(f)

	waitFor(t, func() bool {
		_, err := os.Stat(f)
		return os.IsNotExist(err)
	}, 2*time.Second)

	if upserts.Load() != 1 {
		t.Fatalf("expected 1 upsert, got %d", upserts.Load())
	}
}

func TestNonMarkdownUploadsThenCreatesNoteAndDeletesFile(t *testing.T) {
	var uploads atomic.Int64
	var upserts atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/file/upload":
			uploads.Add(1)
			_ = json.NewEncoder(w).Encode(blinko.UploadResponse{Path: "/api/file/a", Name: "a.bin", Size: 3, Type: "application/octet-stream"})
		case "/api/v1/note/upsert":
			upserts.Add(1)
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	f := filepath.Join(dir, "a.bin")
	mustWrite(t, f, "abc")

	failedDir := filepath.Join(dir, "failed")
	q, cancel := newTestQueue(t, srv.URL, failedDir, true, "", 1, 2, 10*time.Millisecond)
	defer cancel()
	q.EnqueuePath(f)

	waitFor(t, func() bool {
		_, err := os.Stat(f)
		return os.IsNotExist(err)
	}, 2*time.Second)

	if uploads.Load() != 1 {
		t.Fatalf("expected 1 upload, got %d", uploads.Load())
	}
	if upserts.Load() != 1 {
		t.Fatalf("expected 1 upsert, got %d", upserts.Load())
	}
}

func TestUploadFailureRetriesThenQuarantine(t *testing.T) {
	var uploads atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/file/upload" {
			uploads.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("boom"))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	dir := t.TempDir()
	f := filepath.Join(dir, "a.bin")
	mustWrite(t, f, "abc")
	failedDir := filepath.Join(dir, "failed")

	q, cancel := newTestQueue(t, srv.URL, failedDir, true, "", 1, 2, 10*time.Millisecond)
	defer cancel()
	q.EnqueuePath(f)

	failedFile := filepath.Join(failedDir, "a.bin")
	waitFor(t, func() bool {
		_, err := os.Stat(failedFile)
		return err == nil
	}, 3*time.Second)
	waitFor(t, func() bool {
		_, err := os.Stat(failedFile + ".error.json")
		return err == nil
	}, 2*time.Second)

	if uploads.Load() != 3 {
		t.Fatalf("expected 3 upload attempts, got %d", uploads.Load())
	}
}

func TestUpsertFailureAfterUploadRetriesThenQuarantine(t *testing.T) {
	var uploads atomic.Int64
	var upserts atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/file/upload":
			uploads.Add(1)
			_ = json.NewEncoder(w).Encode(blinko.UploadResponse{Path: "/api/file/a", Name: "a.bin", Size: 3, Type: "application/octet-stream"})
		case "/api/v1/note/upsert":
			upserts.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("upsert failed"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	f := filepath.Join(dir, "a.bin")
	mustWrite(t, f, "abc")
	failedDir := filepath.Join(dir, "failed")

	q, cancel := newTestQueue(t, srv.URL, failedDir, true, "", 1, 2, 10*time.Millisecond)
	defer cancel()
	q.EnqueuePath(f)

	failedFile := filepath.Join(failedDir, "a.bin")
	waitFor(t, func() bool {
		_, err := os.Stat(failedFile)
		return err == nil
	}, 3*time.Second)

	if uploads.Load() != 3 || upserts.Load() != 3 {
		t.Fatalf("expected 3 upload/upsert attempts, got uploads=%d upserts=%d", uploads.Load(), upserts.Load())
	}
}

func newTestQueue(t *testing.T, baseURL, failedDir string, deleteOnOK bool, archiveDir string, workers, maxRetries int, retryDelay time.Duration) (*Queue, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	client := blinko.New(baseURL, "token", &http.Client{Timeout: 2 * time.Second})
	proc := processor.New(client, processor.Config{DeleteOnOK: deleteOnOK, ArchiveDir: archiveDir, FailedDir: failedDir})
	q := New(128, workers, maxRetries, retryDelay, proc, metrics.New(), store.NewDedupe(5*time.Millisecond), func(string, ...any) {})
	go func() {
		_ = q.Run(ctx)
	}()
	return q, cancel
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func waitFor(t *testing.T, cond func() bool, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("condition not met within %s", timeout)
}
