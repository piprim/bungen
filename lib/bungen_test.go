package bungen

import (
	"log"
	"os"
	"testing"

	"github.com/LdDl/bungen/internal/testdb"
)

func prepareReq(t testing.TB) (url string, logger *log.Logger) {
	t.Helper()
	logger = log.New(os.Stderr, "", log.LstdFlags)
	url = testdb.DSN(t)

	return
}

func TestBungen_Read(t *testing.T) {
	bungenCLI := New(prepareReq(t))

	t.Run("Should read DB", func(t *testing.T) {
		entities, err := bungenCLI.Read([]string{"public.*"}, true, false, nil)
		if err != nil {
			t.Errorf("Bungen.Read error %v", err)
			return
		}

		if ln := len(entities); ln != 3 {
			t.Errorf("len(entities) = %v, want %v", ln, 3)
			return
		}
	})
}
