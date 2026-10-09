// Package testutil provides the kernel fixture module tests run against.
package testutil

import (
	"github.com/aristorinjuang/lesstruct/kernel"
	themedefault "github.com/aristorinjuang/lesstruct/modules/theme-default"
)

// NewModule builds the theme-default module.
func NewModule() *themedefault.Module {
	return themedefault.New()
}

// Kernel returns a bare host for Init testing.
func Kernel() kernel.Kernel {
	return kernel.NewHost(nil)
}
