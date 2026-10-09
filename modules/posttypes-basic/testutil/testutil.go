// Package testutil provides the kernel fixture module tests run against:
// an in-memory post-type registry with no external dependencies.
package testutil

import (
	"github.com/aristorinjuang/lesstruct/kernel"
	posttypesbasic "github.com/aristorinjuang/lesstruct/modules/posttypes-basic"
	posttype "github.com/aristorinjuang/lesstruct/modules/posttypes-basic/domain"
)

type discardLogger struct{}

func (discardLogger) Info(string, ...any) {}

func (discardLogger) Error(string, ...any) {}

// NewModule builds a module with a fresh default registry and no resolver
// (legacy allow-all scoping). Tests needing role scoping pass
// WithCapabilityResolver via opts.
func NewModule(opts ...posttypesbasic.Option) *posttypesbasic.Module {
	base := []posttypesbasic.Option{
		posttypesbasic.WithService(posttype.NewService()),
	}
	return posttypesbasic.New(append(base, opts...)...)
}

// AllowAll is a resolver stub granting every capability to every principal.
func AllowAll(kernel.Principal, string) bool {
	return true
}

// Kernel returns a bare host for Init testing.
func Kernel() kernel.Kernel {
	return kernel.NewHost(discardLogger{})
}
