package capability_test

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel/capability"
	"github.com/stretchr/testify/assert"
)

func TestManageContent(t *testing.T) {
	assert.Equal(t, "content.manage:post", capability.ManageContent("post"))
	assert.Equal(t, "content.manage:page", capability.ManageContent("page"))
}

func TestSplit(t *testing.T) {
	tests := []struct {
		name         string
		capability   string
		expectedName capability.Capability
		expectedArg  string
	}{
		{
			name:         "bare name has no argument",
			capability:   "admin",
			expectedName: capability.CapabilityAdmin,
			expectedArg:  "",
		},
		{
			name:         "typed capability splits",
			capability:   "content.manage:post",
			expectedName: capability.CapabilityManageContent,
			expectedArg:  "post",
		},
		{
			name:         "unknown name passes through",
			capability:   "site.destroy",
			expectedName: capability.Capability("site.destroy"),
			expectedArg:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, arg := capability.Split(tt.capability)
			assert.Equal(t, tt.expectedName, name)
			assert.Equal(t, tt.expectedArg, arg)
		})
	}
}
