// Package testutil provides the kernel fixture module tests run against.
package testutil

import (
	"github.com/aristorinjuang/lesstruct/kernel"
	sitessr "github.com/aristorinjuang/lesstruct/modules/site-ssr"
)

// NewModule builds the site-ssr module.
func NewModule() *sitessr.Module {
	return sitessr.New()
}

// Kernel returns a bare host for Init testing.
func Kernel() kernel.Kernel {
	return kernel.NewHost(nil)
}
