package cmd

import (
	"context"
	"fmt"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/aristorinjuang/lesstruct/pkg/apiv1"
)

// defaultDownloadTimeout caps one archive pull end to end; large exports of
// big media libraries need headroom, while an unbounded pull lets a
// hostile server hang the operator's terminal forever.
const defaultDownloadTimeout = "30m"

// defaultMaxDownloadSize caps one archive body; multi-gigabyte sites raise
// it explicitly instead of silently pulling forever.
const defaultMaxDownloadSize = "2GiB"

// parseDownloadOptions resolves the --timeout/--max-download-size flag pair
// into client bounds. Empty strings fall back to the documented defaults; a
// zero value disables that bound (documented escape hatch, not an accident:
// omitting the flag keeps the default).
func parseDownloadOptions(timeout, maxSize string) (client.DownloadOptions, *exitError) {
	if timeout == "" {
		timeout = defaultDownloadTimeout
	}
	if maxSize == "" {
		maxSize = defaultMaxDownloadSize
	}
	dur, err := time.ParseDuration(timeout)
	if err != nil || dur < 0 {
		return client.DownloadOptions{}, &exitError{
			code: client.ExitUsage,
			msg:  fmt.Sprintf("lesstruct-cli: invalid --timeout %q (want a Go duration like 30m, 2h, or 0 to disable)", timeout),
		}
	}
	size, err := parseByteSize(maxSize)
	if err != nil {
		return client.DownloadOptions{}, &exitError{
			code: client.ExitUsage,
			msg:  fmt.Sprintf("lesstruct-cli: invalid --max-download-size %q (want bytes or a K/M/G suffixed value like 500MB, or 0 to disable)", maxSize),
		}
	}
	return client.DownloadOptions{Timeout: dur, MaxBytes: size}, nil
}

// parseByteSize parses a byte count with optional K/KB/KiB, M/MB/MiB,
// G/GB/GiB suffixes (case-insensitive, powers of 1024). Bare numbers are
// bytes. Zero disables the cap; negatives and garbage are usage errors.
func parseByteSize(raw string) (int64, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, fmt.Errorf("empty value")
	}
	mult := int64(1)
	upper := strings.ToUpper(s)
	for _, suffix := range []struct {
		names []string
		scale int64
	}{
		{[]string{"GB", "GIB", "G"}, 1 << 30},
		{[]string{"MB", "MIB", "M"}, 1 << 20},
		{[]string{"KB", "KIB", "K"}, 1 << 10},
		{[]string{"B"}, 1},
	} {
		for _, name := range suffix.names {
			if strings.HasSuffix(upper, name) {
				mult = suffix.scale
				s = strings.TrimSpace(s[:len(s)-len(name)])
				break
			}
		}
		if mult != 1 {
			break
		}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("not a non-negative byte count: %q", raw)
	}
	if mult != 1 && n > (1<<63-1)/mult {
		return 0, fmt.Errorf("byte count overflows: %q", raw)
	}
	return n * mult, nil
}

// signalContext cancels the command context on SIGINT/SIGTERM so an
// interrupted download fails fast through the client's cancellable request:
// the caller takes the error path and removes the partial file instead of
// leaving a truncated archive behind. SIGKILL cannot be trapped — document,
// don't pretend otherwise.
func signalContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
}
