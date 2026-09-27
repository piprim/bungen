package validate

import (
	"os"
	"testing"

	"github.com/LdDl/bungen/internal/testdb"
)

func TestMain(m *testing.M) {
	os.Exit(testdb.Run(m))
}
