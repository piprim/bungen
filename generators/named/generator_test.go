package named

import (
	"github.com/LdDl/bungen/internal/testdb"
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"testing"
)

func TestGenerator_Generate(t *testing.T) {
	generator := New()
	options := generator.Options()

	options.Def()
	options.URL = testdb.DSN()
	options.Output = path.Join(t.TempDir(), "model_test.go")
	options.FollowFKs = true

	generator.SetOptions(options)

	if err := generator.Generate(); err != nil {
		t.Errorf("generate error = %v", err)
		return
	}

	generated, err := ioutil.ReadFile(options.Output)
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
