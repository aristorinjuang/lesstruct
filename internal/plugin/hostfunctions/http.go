package hostfunctions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/aristorinjuang/lesstruct/internal/plugin/capability"
	"github.com/tetratelabs/wazero/api"
)

const (
	maxResponseBody = 1 * 1024 * 1024 // 1MB
	resultBufOffset = 4096            // write results starting at this offset in WASM memory
)

type httpCallResult struct {
	Status  int               `json:"status"`
	Body    string            `json:"body,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Error   string            `json:"error,omitempty"`
	Message string            `json:"message,omitempty"`
}

// httpGet is the host function for lesstruct.http_get.
// WASM signature: (url_ptr: i32, url_len: i32, headers_json_ptr: i32, headers_json_len: i32) -> result_offset: i32
func httpGet(
	manifest *capability.Manifest,
	client *http.Client,
	logger func(string),
) api.GoModuleFunc {
	return api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
		urlPtr := api.DecodeU32(stack[0])
		urlLen := api.DecodeU32(stack[1])
		headersPtr := api.DecodeU32(stack[2])
		headersLen := api.DecodeU32(stack[3])

		mem := mod.Memory()
		if mem == nil {
			stack[0] = writeErrorResult(mem, "no_memory", "plugin has no memory")
			return
		}

		urlBytes, ok := mem.Read(urlPtr, urlLen)
		if !ok {
			stack[0] = writeErrorResult(mem, "memory_error", "failed to read url from memory")
			return
		}
		url := string(urlBytes)

		// Validate against manifest
		if !manifest.IsHTTPURLAllowed(url) {
			logger(fmt.Sprintf("HTTP GET blocked: url %q not in allowlist", url))
			stack[0] = writeErrorResult(mem, "url_not_allowed",
				fmt.Sprintf("URL %q not in capability manifest http allowlist", url))
			return
		}

		// Read optional headers
		var headers map[string]string
		if headersLen > 0 {
			headersBytes, ok := mem.Read(headersPtr, headersLen)
			if ok && len(headersBytes) > 0 {
				_ = json.Unmarshal(headersBytes, &headers)
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			stack[0] = writeErrorResult(mem, "request_error", err.Error())
			return
		}

		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := client.Do(req)
		if err != nil {
			stack[0] = writeErrorResult(mem, "http_error", err.Error())
			return
		}
		defer func() { _ = resp.Body.Close() }()

		limited := io.LimitReader(resp.Body, maxResponseBody)
		bodyBytes, err := io.ReadAll(limited)
		if err != nil {
			stack[0] = writeErrorResult(mem, "read_error", err.Error())
			return
		}

		respHeaders := make(map[string]string)
		for k := range resp.Header {
			respHeaders[k] = resp.Header.Get(k)
		}

		result := httpCallResult{
			Status:  resp.StatusCode,
			Body:    string(bodyBytes),
			Headers: respHeaders,
		}

		stack[0] = writeJSONResult(mem, result)
	})
}

// httpPost is the host function for lesstruct.http_post.
// WASM signature: (url_ptr, url_len, headers_json_ptr, headers_json_len, body_ptr, body_len) -> result_offset: i32
func httpPost(
	manifest *capability.Manifest,
	client *http.Client,
	logger func(string),
) api.GoModuleFunc {
	return api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
		urlPtr := api.DecodeU32(stack[0])
		urlLen := api.DecodeU32(stack[1])
		headersPtr := api.DecodeU32(stack[2])
		headersLen := api.DecodeU32(stack[3])
		bodyPtr := api.DecodeU32(stack[4])
		bodyLen := api.DecodeU32(stack[5])

		mem := mod.Memory()
		if mem == nil {
			stack[0] = writeErrorResult(mem, "no_memory", "plugin has no memory")
			return
		}

		urlBytes, ok := mem.Read(urlPtr, urlLen)
		if !ok {
			stack[0] = writeErrorResult(mem, "memory_error", "failed to read url from memory")
			return
		}
		url := string(urlBytes)

		if !manifest.IsHTTPURLAllowed(url) {
			logger(fmt.Sprintf("HTTP POST blocked: url %q not in allowlist", url))
			stack[0] = writeErrorResult(mem, "url_not_allowed",
				fmt.Sprintf("URL %q not in capability manifest http allowlist", url))
			return
		}

		var headers map[string]string
		if headersLen > 0 {
			headersBytes, ok := mem.Read(headersPtr, headersLen)
			if ok && len(headersBytes) > 0 {
				_ = json.Unmarshal(headersBytes, &headers)
			}
		}

		var bodyReader io.Reader
		if bodyLen > 0 {
			bodyBytes, ok := mem.Read(bodyPtr, bodyLen)
			if ok {
				bodyReader = strings.NewReader(string(bodyBytes))
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bodyReader)
		if err != nil {
			stack[0] = writeErrorResult(mem, "request_error", err.Error())
			return
		}

		for k, v := range headers {
			req.Header.Set(k, v)
		}
		if req.Header.Get("Content-Type") == "" && bodyReader != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := client.Do(req)
		if err != nil {
			stack[0] = writeErrorResult(mem, "http_error", err.Error())
			return
		}
		defer func() { _ = resp.Body.Close() }()

		limited := io.LimitReader(resp.Body, maxResponseBody)
		bodyBytes, err := io.ReadAll(limited)
		if err != nil {
			stack[0] = writeErrorResult(mem, "read_error", err.Error())
			return
		}

		respHeaders := make(map[string]string)
		for k := range resp.Header {
			respHeaders[k] = resp.Header.Get(k)
		}

		result := httpCallResult{
			Status:  resp.StatusCode,
			Body:    string(bodyBytes),
			Headers: respHeaders,
		}

		stack[0] = writeJSONResult(mem, result)
	})
}

func writeJSONResult(mem api.Memory, v any) uint64 {
	data, err := json.Marshal(v)
	if err != nil {
		return writeErrorResult(mem, "json_error", err.Error())
	}
	if len(data) > maxResponseBody {
		return writeErrorResult(mem, "response_too_large", "response exceeds 1MB limit")
	}
	mem.Write(resultBufOffset, data)
	return api.EncodeU32(resultBufOffset)
}

func writeErrorResult(mem api.Memory, errCode string, message string) uint64 {
	result := httpCallResult{
		Error:   errCode,
		Message: message,
	}
	data, _ := json.Marshal(result)
	if mem != nil {
		mem.Write(resultBufOffset, data)
	}
	return api.EncodeU32(resultBufOffset)
}

