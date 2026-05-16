package postgres

import (
    "context"

    "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
    db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
    return &Repository{db: db}
}

// Transaction handling
type txKey struct{}

func (r *Repository) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
    tx, err := r.db.Begin(ctx)
    if err != nil {
        return err
    }
	defer func() {
		_ = tx.Rollback(ctx)
	}()

    // Store tx in context
    ctx = context.WithValue(ctx, txKey{}, tx)

    if err := fn(ctx); err != nil {
        return err
    }

    return tx.Commit(ctx)
}

// Helper to get tx or pool
func (r *Repository) getQueryExec(ctx context.Context) PgxQueryExec {
    if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
        return tx
    }
    return r.db
}

// Interface for both pool and tx
type PgxQueryExec interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

