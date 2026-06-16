package runtime

import (
	"time"
)

const (
	DefaultMaxMemory     = 64 * 1024 * 1024 // 64MB
	DefaultMaxExecTime   = 30 * time.Second
	defaultWazeroVersion = "1.11.0"
)

type RuntimeConfig struct {
	MaxMemoryBytes   uint64
	MaxExecutionTime time.Duration
	Logger           func(string)
}
