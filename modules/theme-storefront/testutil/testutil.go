// Package testutil provides the kernel fixture module tests run against.
package testutil

import (
	"github.com/aristorinjuang/lesstruct/kernel"
	themestorefront "github.com/aristorinjuang/lesstruct/modules/theme-storefront"
)

// NewModule builds the theme-storefront module.
func NewModule() *themestorefront.Module {
	return themestorefront.New()
}

// Kernel returns a bare host for Init testing.
func Kernel() kernel.Kernel {
	return kernel.NewHost(nil)
}
