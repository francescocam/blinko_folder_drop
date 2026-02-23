package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIsStable(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "a.txt")
	if err := os.WriteFile(p, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	ok, err := IsStable(p, 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("expected stable file")
	}
}
