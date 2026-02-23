package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadValidation(t *testing.T) {
	d := t.TempDir()
	cfgPath := filepath.Join(d, "cfg.yaml")
	if err := os.WriteFile(cfgPath, []byte("blinko:\n  base_url: \"\"\n  jwt_token: \"\"\nwatch:\n  input_dir: \"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(cfgPath); err == nil {
		t.Fatalf("expected validation error")
	}
}
