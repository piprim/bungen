package dump

import (
	"bufio"
	"context"
	"database/sql"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

// Dump selects the rows cfg describes from db and writes them to w as a psql
// data script; source names the config in the script header. The row count of
// every table goes to summary.
func Dump(ctx context.Context, db *bun.DB, cfg *Config, source string, w, summary io.Writer) error {
	return withSnapshot(ctx, db, func(conn bun.Conn) error {
		cat, err := loadCatalog(ctx, conn.Conn)
		if err != nil {
			return err
		}
		rows, err := walk(ctx, conn.Conn, cat, cfg)
		if err != nil {
			return err
		}
		rows.summary(summary)

		bw := bufio.NewWriter(w)
		if err := writeData(ctx, conn, bw, cat, rows, source); err != nil {
			return err
		}
		return bw.Flush()
	})
}

// CreateCommand creates the dump command.
func CreateCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "dump",
		Short: "Dump a consistent subset of a database as a psql data script",
		Long: `Selects the seed rows of a config file, their children and every row they
reference, and writes them as COPY blocks that psql loads into an empty
database with the same schema. See dump/README.md.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SilenceUsage = true
			flags := cmd.Flags()
			conn, _ := flags.GetString("conn")
			config, _ := flags.GetString("config")
			output, _ := flags.GetString("output")
			return run(context.Background(), conn, config, output, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}

	flags := command.Flags()
	flags.SortFlags = false
	flags.StringP("conn", "c", "", "connection string to postgres database, e.g. postgres://user:password@localhost:5432/db?sslmode=disable")
	flags.String("config", "", "dump configuration file")
	flags.StringP("output", "o", "", "output file; stdout when empty")
	_ = command.MarkFlagRequired("conn")
	_ = command.MarkFlagRequired("config")

	return command
}

// run dumps to output through a temporary file renamed on success, so a failed
// dump never leaves a truncated script or clobbers the previous one.
func run(ctx context.Context, url, configPath, output string, stdout, stderr io.Writer) error {
	f, err := os.Open(configPath)
	if err != nil {
		return err
	}
	cfg, err := ParseConfig(f)
	_ = f.Close()
	if err != nil {
		return err
	}

	db := connect(url)
	defer func() { _ = db.Close() }()

	if output == "" {
		return Dump(ctx, db, cfg, configPath, stdout, stderr)
	}

	tmp := output + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if err := Dump(ctx, db, cfg, configPath, out, stderr); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, output)
}

// connect opens the database the dump reads. pgdriver's default 10 s read
// timeout would cut a long COPY or child query short, and a dump has no
// interactive caller to protect, so it reads without a deadline.
func connect(url string) *bun.DB {
	connector := pgdriver.NewConnector(pgdriver.WithDSN(url), pgdriver.WithReadTimeout(0))
	return bun.NewDB(sql.OpenDB(connector), pgdialect.New())
}
