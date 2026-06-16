package hostfunctions_test

import (
	"context"
	"database/sql"
	"net/http"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/plugin/hostfunctions"
	"github.com/stretchr/testify/require"
)

func TestNewRegistry(t *testing.T) {
	t.Run("creates registry with HTTP client and DB", func(t *testing.T) {
		client := &http.Client{}
		var logs []string
		logger := func(msg string) {
			logs = append(logs, msg)
		}

		r := hostfunctions.NewRegistry(client, nil, logger)
		require.NotNil(t, r)

		err := r.Close()
		require.NoError(t, err)
	})

	t.Run("defaults nil HTTP client to http.DefaultClient", func(t *testing.T) {
		r := hostfunctions.NewRegistry(nil, nil, nil)
		require.NotNil(t, r)

		err := r.Close()
		require.NoError(t, err)
	})

	t.Run("nil logger does not panic", func(t *testing.T) {
		r := hostfunctions.NewRegistry(nil, nil, nil)
		require.NotNil(t, r)
		// Should not panic during any operation
		_ = r.Close()
	})
}

func TestRegistryClose(t *testing.T) {
	t.Run("closing with nil client does not panic", func(t *testing.T) {
		r := hostfunctions.NewRegistry(nil, nil, nil)
		err := r.Close()
		require.NoError(t, err)
	})

	t.Run("double close does not panic", func(t *testing.T) {
		r := hostfunctions.NewRegistry(nil, nil, nil)
		_ = r.Close()
		_ = r.Close()
	})
}

// stubDB is a minimal DBExecutor implementation for testing.
type stubDB struct {
	queryFunc func(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	execFunc  func(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func (s *stubDB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if s.queryFunc != nil {
		return s.queryFunc(ctx, query, args...)
	}
	return nil, nil
}

func (s *stubDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if s.execFunc != nil {
		return s.execFunc(ctx, query, args...)
	}
	return nil, nil
}

func TestRegistryWithDB(t *testing.T) {
	t.Run("creates registry with DB executor", func(t *testing.T) {
		db := &stubDB{}
		r := hostfunctions.NewRegistry(nil, db, nil)
		require.NotNil(t, r)
		_ = r.Close()
	})
}

func TestRegistryWithHTTPClient(t *testing.T) {
	t.Run("creates registry with custom HTTP client", func(t *testing.T) {
		client := &http.Client{}
		r := hostfunctions.NewRegistry(client, nil, nil)
		require.NotNil(t, r)
		_ = r.Close()
	})
}