package hostfunctions

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aristorinjuang/lesstruct/internal/plugin/capability"
	"github.com/tetratelabs/wazero/api"
)

type dbResult struct {
	Columns []string `json:"columns,omitempty"`
	Rows    [][]any  `json:"rows,omitempty"`
	Error   string   `json:"error,omitempty"`
	Message string   `json:"message,omitempty"`
}

// dbQuery is the host function for lesstruct.db_query.
// WASM signature: (sql_ptr: i32, sql_len: i32, params_json_ptr: i32, params_json_len: i32) -> result_offset: i32
func dbQuery(
	manifest *capability.Manifest,
	db DBExecutor,
	logger func(string),
) api.GoModuleFunc {
	return api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
		sqlPtr := api.DecodeU32(stack[0])
		sqlLen := api.DecodeU32(stack[1])
		paramsPtr := api.DecodeU32(stack[2])
		paramsLen := api.DecodeU32(stack[3])

		mem := mod.Memory()
		if mem == nil {
			stack[0] = writeDBError(mem, "no_memory", "plugin has no memory")
			return
		}

		sqlBytes, ok := mem.Read(sqlPtr, sqlLen)
		if !ok {
			stack[0] = writeDBError(mem, "memory_error", "failed to read SQL from memory")
			return
		}
		query := string(sqlBytes)

		if err := checkDBPermission(manifest, query, "read"); err != nil {
			logger(fmt.Sprintf("DB query blocked: %v", err))
			stack[0] = writeDBError(mem, "permission_denied", err.Error())
			return
		}

		var params []any
		if paramsLen > 0 {
			paramsBytes, ok := mem.Read(paramsPtr, paramsLen)
			if ok && len(paramsBytes) > 0 {
				var rawParams []any
				if err := json.Unmarshal(paramsBytes, &rawParams); err == nil {
					params = rawParams
				}
			}
		}

		rows, err := db.QueryContext(ctx, query, params...)
		if err != nil {
			stack[0] = writeDBError(mem, "query_error", err.Error())
			return
		}
		defer func() { _ = rows.Close() }()

		columns, err := rows.Columns()
		if err != nil {
			stack[0] = writeDBError(mem, "columns_error", err.Error())
			return
		}

		var resultRows [][]any
		for rows.Next() {
			values := make([]any, len(columns))
			valuePtrs := make([]any, len(columns))
			for i := range values {
				valuePtrs[i] = &values[i]
			}

			if err := rows.Scan(valuePtrs...); err != nil {
				stack[0] = writeDBError(mem, "scan_error", err.Error())
				return
			}

			// Convert to JSON-safe types
			safeValues := make([]any, len(values))
			for i, v := range values {
				safeValues[i] = convertToJSONSafe(v)
			}
			resultRows = append(resultRows, safeValues)
		}

		if err := rows.Err(); err != nil {
			stack[0] = writeDBError(mem, "rows_error", err.Error())
			return
		}

		result := dbResult{
			Columns: columns,
			Rows:    resultRows,
		}

		stack[0] = writeJSONResult(mem, result)
	})
}

// dbExec is the host function for lesstruct.db_exec.
// WASM signature: (sql_ptr: i32, sql_len: i32, params_json_ptr: i32, params_json_len: i32) -> result_offset: i32
func dbExec(
	manifest *capability.Manifest,
	db DBExecutor,
	logger func(string),
) api.GoModuleFunc {
	return api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
		sqlPtr := api.DecodeU32(stack[0])
		sqlLen := api.DecodeU32(stack[1])
		paramsPtr := api.DecodeU32(stack[2])
		paramsLen := api.DecodeU32(stack[3])

		mem := mod.Memory()
		if mem == nil {
			stack[0] = writeDBError(mem, "no_memory", "plugin has no memory")
			return
		}

		sqlBytes, ok := mem.Read(sqlPtr, sqlLen)
		if !ok {
			stack[0] = writeDBError(mem, "memory_error", "failed to read SQL from memory")
			return
		}
		query := string(sqlBytes)

		if err := checkDBPermission(manifest, query, "write"); err != nil {
			logger(fmt.Sprintf("DB exec blocked: %v", err))
			stack[0] = writeDBError(mem, "permission_denied", err.Error())
			return
		}

		var params []any
		if paramsLen > 0 {
			paramsBytes, ok := mem.Read(paramsPtr, paramsLen)
			if ok && len(paramsBytes) > 0 {
				var rawParams []any
				if err := json.Unmarshal(paramsBytes, &rawParams); err == nil {
					params = rawParams
				}
			}
		}

		result, err := db.ExecContext(ctx, query, params...)
		if err != nil {
			stack[0] = writeDBError(mem, "exec_error", err.Error())
			return
		}

		rowsAffected, _ := result.RowsAffected()
		lastInsertID, _ := result.LastInsertId()

		execResult := map[string]any{
			"rows_affected":  rowsAffected,
			"last_insert_id": lastInsertID,
		}

		stack[0] = writeJSONResult(mem, execResult)
	})
}

func normalizeTableName(table string) string {
	switch table {
	case "content_items":
		return "content"
	case "media_files":
		return "media"
	default:
		return table
	}
}

func checkDBPermission(manifest *capability.Manifest, query string, operation string) error {
	tableName := extractTableName(query)
	if tableName == "" {
		return fmt.Errorf("could not determine table name from query")
	}

	tableName = normalizeTableName(tableName)

	requiredPerm := fmt.Sprintf("%s:%s", operation, tableName)
	if manifest.HasDBPermission(requiredPerm) {
		return nil
	}

	return fmt.Errorf(
		"plugin does not have permission %q for table %q",
		requiredPerm,
		tableName,
	)
}

func extractTableName(query string) string {
	query = strings.TrimSpace(query)
	upper := strings.ToUpper(query)

	// SELECT ... FROM table
	if idx := strings.Index(upper, " FROM "); idx >= 0 {
		rest := query[idx+6:] // after " FROM "
		return firstWord(rest)
	}

	// INSERT INTO table
	if idx := strings.Index(upper, "INSERT INTO "); idx >= 0 {
		rest := query[idx+12:] // after "INSERT INTO "
		return firstWord(rest)
	}

	// UPDATE table
	if idx := strings.Index(upper, "UPDATE "); idx >= 0 {
		rest := query[idx+7:] // after "UPDATE "
		return firstWord(rest)
	}

	// DELETE FROM table
	if idx := strings.Index(upper, "DELETE FROM "); idx >= 0 {
		rest := query[idx+12:] // after "DELETE FROM "
		return firstWord(rest)
	}

	return ""
}

func firstWord(s string) string {
	s = strings.TrimSpace(s)
	idx := strings.IndexAny(s, " \t\n\r(")
	if idx < 0 {
		return strings.ToLower(strings.TrimSpace(s))
	}
	return strings.ToLower(strings.TrimSpace(s[:idx]))
}

func convertToJSONSafe(v any) any {
	switch val := v.(type) {
	case []byte:
		return string(val)
	case sql.NullString:
		if val.Valid {
			return val.String
		}
		return nil
	case sql.NullInt64:
		if val.Valid {
			return val.Int64
		}
		return nil
	case sql.NullFloat64:
		if val.Valid {
			return val.Float64
		}
		return nil
	case sql.NullBool:
		if val.Valid {
			return val.Bool
		}
		return nil
	default:
		return v
	}
}

func writeDBError(mem api.Memory, errCode string, message string) uint64 {
	result := dbResult{
		Error:   errCode,
		Message: message,
	}
	data, _ := json.Marshal(result)
	if mem != nil {
		mem.Write(resultBufOffset, data)
	}
	return api.EncodeU32(resultBufOffset)
}

// DBExecutor abstracts database operations for host functions.
type DBExecutor interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}
