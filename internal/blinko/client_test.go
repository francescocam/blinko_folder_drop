package blinko

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestUploadAndUpsert(t *testing.T) {
	var authHeaders []string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeaders = append(authHeaders, r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/api/file/upload":
			_ = json.NewEncoder(w).Encode(UploadResponse{Path: "/api/file/x", Name: "x", Size: 1, Type: "text/plain"})
		case "/api/v1/note/upsert":
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer s.Close()

	client := New(s.URL, "abc", s.Client())
	d := t.TempDir()
	p := filepath.Join(d, "x.txt")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := client.UploadFile(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	if err := client.UpsertNote(context.Background(), NoteUpsertRequest{Content: "c", Type: -1}); err != nil {
		t.Fatal(err)
	}
	if len(authHeaders) != 2 || authHeaders[0] != "Bearer abc" || authHeaders[1] != "Bearer abc" {
		t.Fatalf("unexpected auth headers: %+v", authHeaders)
	}
}
