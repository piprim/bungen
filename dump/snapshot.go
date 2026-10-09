package dump

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// beginSnapshot opens the transaction every dump query runs in. The settings
// make COPY output independent of the server's defaults.
const beginSnapshot = `BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY;
SET TimeZone = 'UTC';
SET DateStyle = 'ISO, YMD';
SET IntervalStyle = 'postgres';
SET extra_float_digits = 3`

// withSnapshot runs fn on one connection inside a read-only repeatable-read
// transaction, so every query sees the same data and ctids stay valid.
func withSnapshot(ctx context.Context, db *bun.DB, fn func(conn bun.Conn) error) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("dump: connect: %w", err)
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.Conn.ExecContext(ctx, beginSnapshot); err != nil {
		return fmt.Errorf("dump: begin snapshot: %w", err)
	}
	defer func() { _, _ = conn.Conn.ExecContext(context.Background(), "ROLLBACK") }()

	return fn(conn)
}
