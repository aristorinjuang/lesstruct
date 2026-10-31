// Package testutil provides the kernel fixture module tests run against.
package testutil

import (
	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/aristorinjuang/lesstruct/modules/cart"
)

// NewModule builds the cart module bare (no options): the fixture for
// lifecycle and failure-path testing.
func NewModule() *cart.Module {
	return cart.New()
}

// Kernel returns a bare host for Init testing.
func Kernel() kernel.Kernel {
	return kernel.NewHost(nil)
}
