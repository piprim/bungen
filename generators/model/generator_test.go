package model

import (
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"testing"

	"github.com/LdDl/bungen/internal/testdb"
	"github.com/LdDl/bungen/model"
)

func TestGenerator_Generate(t *testing.T) {
	generator := New()

	generator.options.Def()
	generator.options.URL = testdb.DSN(t)
	generator.options.Output = path.Join(t.TempDir(), "model_test.go")
	generator.options.FollowFKs = true
	generator.options.GenORM = true
	generator.options.DBWrapName = "MyCustomWrapper"
	generator.options.CustomTypes.Add(model.TypePGUuid, "uuid.UUID", "github.com/google/uuid")
	//generator.options.AddJSONTag = true

	if err := generator.Generate(); err != nil {
		t.Errorf("generate error = %v", err)
		return
	}

	generated, err := ioutil.ReadFile(generator.options.Output)
	if err != nil {
		t.Errorf("file not generated = %v", err)
	}

	_, filename, _, _ := runtime.Caller(0)
	checkPath := path.Join(path.Dir(filename), "generator_test.output")
	if os.Getenv("BUNGEN_UPDATE_GOLDEN") != "" {
		if err := ioutil.WriteFile(checkPath, generated, 0o644); err != nil {
			t.Fatalf("update golden file = %v", err)
		}
	}
	check, err := ioutil.ReadFile(checkPath)
	if err != nil {
		t.Errorf("check file not found = %v", err)
	}

	if string(generated) != string(check) {
		t.Errorf("generated does not match with check")
		return
	}
}

func TestReadFlags_PresenceDropsDefaultJSONType(t *testing.T) {
	read := func(t *testing.T, args ...string) Options {
		g := New()
		cmd := CreateCommand()
		if err := cmd.ParseFlags(args); err != nil {
			t.Fatalf("parse flags = %v", err)
		}
		if err := g.ReadFlags(cmd); err != nil {
			t.Fatalf("read flags = %v", err)
		}
		return g.options
	}

	t.Run("default wildcard is dropped", func(t *testing.T) {
		opts := read(t, "-c", "x", "-o", "y", "--presence")
		if opts.JSONTypes != nil {
			t.Errorf("JSONTypes = %v, want nil", opts.JSONTypes)
		}
	})

	t.Run("an explicit -j is kept", func(t *testing.T) {
		opts := read(t, "-c", "x", "-o", "y", "--presence", "-j", "a.b=T")
		if got := opts.JSONTypes["a.b"]; got != "T" || len(opts.JSONTypes) != 1 {
			t.Errorf("JSONTypes = %v, want map[a.b:T]", opts.JSONTypes)
		}
	})

	t.Run("without presence the default wildcard stays", func(t *testing.T) {
		opts := read(t, "-c", "x", "-o", "y")
		if got := opts.JSONTypes["*"]; got != "map[string]interface{}" {
			t.Errorf("JSONTypes = %v, want the map default", opts.JSONTypes)
		}
	})
}

func TestGenerator_GeneratePresence(t *testing.T) {
	generator := New()

	generator.options.Def()
	generator.options.URL = testdb.DSN(t)
	generator.options.Output = path.Join(t.TempDir(), "model_presence_test.go")
	generator.options.FollowFKs = true
	generator.options.Presence = true
	generator.options.CustomTypes.Add(model.TypePGUuid, "uuid.UUID", "github.com/google/uuid")

	if err := generator.Generate(); err != nil {
		t.Fatalf("generate error = %v", err)
	}

	generated, err := os.ReadFile(generator.options.Output)
	if err != nil {
		t.Fatalf("file not generated = %v", err)
	}

	_, filename, _, _ := runtime.Caller(0)
	checkPath := path.Join(path.Dir(filename), "generator_test_presence.output")
	if os.Getenv("BUNGEN_UPDATE_GOLDEN") != "" {
		if err := os.WriteFile(checkPath, generated, 0o644); err != nil {
			t.Fatalf("update golden file = %v", err)
		}
	}
	check, err := os.ReadFile(checkPath)
	if err != nil {
		t.Fatalf("check file not found = %v", err)
	}

	t.Run("generated output matches the golden file", func(t *testing.T) {
		if string(generated) != string(check) {
			t.Errorf("generated does not match generator_test_presence.output")
		}
	})
}
