// Package testutil provides the kernel fixture module tests run against.
package testutil

import (
	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/modules/adminpanel"
)

// NewModule builds the adminpanel module. It carries no construction options
// in this slice; later slices extend the fixture alongside the module.
func NewModule() *adminpanel.Module {
	return adminpanel.New()
}

// Kernel returns a bare host for Init testing.
func Kernel() kernel.Kernel {
	return kernel.NewHost(nil)
}
