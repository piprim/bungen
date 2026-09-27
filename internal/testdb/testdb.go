// Package testdb provides a PostgreSQL database for the test suite.
//
// By default it starts a disposable PostgreSQL container with Testcontainers
// and loads test_db.sql from the repository root into it. When the
// BUNGEN_TEST_DSN environment variable is set, no container is started and
// tests connect to that URL instead; the schema from test_db.sql must already
// be loaded there.
package testdb

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// EnvDSN is the environment variable that bypasses the container.
const EnvDSN = "BUNGEN_TEST_DSN"

const (
	image    = "postgres:17-alpine"
	dbName   = "some_db"
	user     = "some_user"
	password = "some_password"
)

var dsn string

// DSN returns the connection URL of the test database.
// It is only valid inside tests run through Run.
func DSN() string {
	if dsn == "" {
		panic("testdb: DSN called before Run; add a TestMain that calls testdb.Run")
	}
	return dsn
}

// Run starts the test database, runs the package's tests and stops the
// database. It is meant to be called from TestMain:
//
//	func TestMain(m *testing.M) { os.Exit(testdb.Run(m)) }
func Run(m *testing.M) int {
	if env := os.Getenv(EnvDSN); env != "" {
		dsn = env
		return m.Run()
	}

	ctx := context.Background()

	container, err := start(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "testdb: %v\n", err)
		return 1
	}

	code := m.Run()

	if err := testcontainers.TerminateContainer(container); err != nil {
		fmt.Fprintf(os.Stderr, "testdb: terminate container: %v\n", err)
	}

	return code
}

func start(ctx context.Context) (container *postgres.PostgresContainer, err error) {
	// Testcontainers panics instead of returning an error when it cannot
	// find a Docker host; turn that into a readable failure.
	defer func() {
		if r := recover(); r != nil {
			container = nil
			err = fmt.Errorf("no Docker host available (%v); start Docker or set %s to an existing database loaded with test_db.sql", r, EnvDSN)
		}
	}()

	container, err = postgres.Run(ctx, image,
		postgres.WithDatabase(dbName),
		postgres.WithUsername(user),
		postgres.WithPassword(password),
		postgres.WithInitScripts(schemaPath()),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		return nil, fmt.Errorf("start postgres container: %w", err)
	}

	url, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = testcontainers.TerminateContainer(container)
		return nil, fmt.Errorf("get connection string: %w", err)
	}
	dsn = url

	return container, nil
}

// schemaPath returns the absolute path of test_db.sql at the repository root.
func schemaPath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "test_db.sql")
}
