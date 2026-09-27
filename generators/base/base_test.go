package base

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LdDl/bungen/model"
)

func TestGenerator_GenerateFromEntities(t *testing.T) {
	packer := func(entities []model.Entity) (interface{}, error) {
		return struct{ Package string }{Package: "."}, nil
	}

	t.Run("Should return error and keep file when generated code does not format", func(t *testing.T) {
		output := filepath.Join(t.TempDir(), "models.go")

		err := Generator{}.GenerateFromEntities(nil, output, "package {{.Package}}\n", packer)
		if err == nil {
			t.Fatalf("GenerateFromEntities() error = nil, want formatting error")
		}
		if !strings.Contains(err.Error(), "formatting") {
			t.Errorf("GenerateFromEntities() error = %q, want it to mention formatting", err)
		}

		content, readErr := os.ReadFile(output)
		if readErr != nil {
			t.Fatalf("unformatted file should still be written, read error = %v", readErr)
		}
		if string(content) != "package .\n" {
			t.Errorf("unformatted content = %q, want %q", content, "package .\n")
		}
	})

	t.Run("Should write formatted code", func(t *testing.T) {
		output := filepath.Join(t.TempDir(), "models.go")

		err := Generator{}.GenerateFromEntities(nil, output, "package   model\n\n\nvar   X   =   1\n", packer)
		if err != nil {
			t.Fatalf("GenerateFromEntities() error = %v", err)
		}

		content, readErr := os.ReadFile(output)
		if readErr != nil {
			t.Fatalf("read error = %v", readErr)
		}
		if want := "package model\n\nvar X = 1\n"; string(content) != want {
			t.Errorf("content = %q, want %q", content, want)
		}
	})
}
