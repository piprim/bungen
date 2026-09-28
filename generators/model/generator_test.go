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
	generator.options.URL = testdb.DSN()
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
