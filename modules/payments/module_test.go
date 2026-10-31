package payments_test

import (
	"testing"

	kernel "github.com/aristorinjuang/lesstruct/kernel"
	paymentsmodule "github.com/aristorinjuang/lesstruct/modules/payments"
	"github.com/stretchr/testify/require"
)

func TestModule_Init(t *testing.T) {
	t.Run("a module without its service fails startup", func(t *testing.T) {
		module := paymentsmodule.New()

		err := module.Init(nil)

		require.Error(t, err)
	})
}

// compile-time witness that the module satisfies the kernel contract.
var _ kernel.Module = (*paymentsmodule.Module)(nil)
