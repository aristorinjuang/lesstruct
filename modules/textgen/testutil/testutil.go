// Package testutil provides the kernel fixture module tests run against.
package testutil

import (
	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/modules/textgen"
)

// NewModule builds the textgen module. It carries no construction options in
// this slice; later slices extend the fixture alongside the module.
func NewModule() *textgen.Module {
	return textgen.New()
}

// Kernel returns a bare host for Init testing.
func Kernel() kernel.Kernel {
	return kernel.NewHost(nil)
}
