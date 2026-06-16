package sdk

// Host function name constants.
// Plugin .wasm files import these from the "lesstruct" host module when
// the corresponding capabilities are declared in the manifest.
const (
	// HostFunctionHTTPGet imports lesstruct.http_get for HTTP GET requests.
	HostFunctionHTTPGet = "http_get"

	// HostFunctionHTTPPost imports lesstruct.http_post for HTTP POST requests.
	HostFunctionHTTPPost = "http_post"

	// HostFunctionDBQuery imports lesstruct.db_query for SELECT queries.
	HostFunctionDBQuery = "db_query"

	// HostFunctionDBExec imports lesstruct.db_exec for INSERT/UPDATE/DELETE
	// statements.
	HostFunctionDBExec = "db_exec"

	// HostFunctionLogInfo imports lesstruct.log_info for informational logging.
	HostFunctionLogInfo = "log_info"

	// HostFunctionLogError imports lesstruct.log_error for error logging.
	HostFunctionLogError = "log_error"
)

// Host module namespace constants.
const (
	// HostModuleName is the WASM import module name for Lesstruct host
	// functions.
	HostModuleName = "lesstruct"
)

// Host function parameter conventions.
//
// All host functions follow a (ptr, len) pair convention for passing
// string / bytes arguments through WASM linear memory:
//
//   - url_ptr, url_len (int32, int32): Address and length of the URL string.
//   - headers_json_ptr, headers_json_len (int32, int32): Optional headers as
//     JSON. Pass (0, 0) if unused.
//   - body_ptr, body_len (int32, int32): Request body bytes.
//   - sql_ptr, sql_len (int32, int32): SQL query string.
//   - params_json_ptr, params_json_len (int32, int32): JSON array of query
//     parameters.
//
// Return values:
//   - result_offset (int32): Offset in WASM memory where the JSON result is
//     written (4096). The caller reads from offset 4096 and checks the
//     returned JSON for {"error": "...", "message": "..."} to detect
//     failures.
//
// Thread safety:
//   - Logging functions (log_info, log_error) are safe to call from any
//     WASM function.
//   - Network and database functions should be called during hook execution
//     (they receive the parent context with timeout/deadline).
//
// See docs/plugin-capabilities.md for detailed usage and examples.