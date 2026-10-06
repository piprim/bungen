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

func TestReadTemplate(t *testing.T) {
	t.Run("Should return the default when no path is set", func(t *testing.T) {
		got, err := ReadTemplate("", "built-in")
		if err != nil {
			t.Fatalf("ReadTemplate() error = %v", err)
		}
		if got != "built-in" {
			t.Errorf("ReadTemplate() = %q, want %q", got, "built-in")
		}
	})

	t.Run("Should return the file content when a path is set", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "model.tmpl")
		if err := os.WriteFile(file, []byte("package {{.Package}}\n"), 0o644); err != nil {
			t.Fatalf("write template = %v", err)
		}

		got, err := ReadTemplate(file, "built-in")
		if err != nil {
			t.Fatalf("ReadTemplate() error = %v", err)
		}
		if want := "package {{.Package}}\n"; got != want {
			t.Errorf("ReadTemplate() = %q, want %q", got, want)
		}
	})

	t.Run("Should return error when the file does not exist", func(t *testing.T) {
		_, err := ReadTemplate(filepath.Join(t.TempDir(), "missing.tmpl"), "built-in")
		if err == nil {
			t.Fatalf("ReadTemplate() error = nil, want a read error")
		}
	})
}
