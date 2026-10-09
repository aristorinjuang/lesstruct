package kernel_test

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrune(t *testing.T) {
	// Mirrors the production Requires graph: users is a leaf; auth and
	// comments require users; adminpanel requires auth.
	candidates := func() []kernel.Module {
		return []kernel.Module{
			newStubModule("users", nil, nil),
			newStubModule("auth", []string{"users"}, nil),
			newStubModule("comments", []string{"users"}, nil),
			newStubModule("adminpanel", []string{"auth"}, nil),
			newStubModule("media", nil, nil),
		}
	}

	tests := []struct {
		name        string
		candidates  []kernel.Module
		disabled    []string
		wantKept    []string
		wantDropped []kernel.DroppedModule
	}{
		{
			name:       "success - empty disabled keeps everything",
			candidates: candidates(),
			disabled:   nil,
			wantKept:   []string{"users", "auth", "comments", "adminpanel", "media"},
		},
		{
			name:       "success - leaf disable drops only itself",
			candidates: candidates(),
			disabled:   []string{"comments"},
			wantKept:   []string{"users", "auth", "adminpanel", "media"},
			wantDropped: []kernel.DroppedModule{
				{Name: "comments", Direct: true, Cause: "comments"},
			},
		},
		{
			name:       "success - users disable cascades auth comments adminpanel",
			candidates: candidates(),
			disabled:   []string{"users"},
			wantKept:   []string{"media"},
			wantDropped: []kernel.DroppedModule{
				{Name: "users", Direct: true, Cause: "users"},
				{Name: "auth", Cause: "users"},
				{Name: "comments", Cause: "users"},
				{Name: "adminpanel", Cause: "auth"},
			},
		},
		{
			name:       "success - auth disable cascades adminpanel only",
			candidates: candidates(),
			disabled:   []string{"auth"},
			wantKept:   []string{"users", "comments", "media"},
			wantDropped: []kernel.DroppedModule{
				{Name: "auth", Direct: true, Cause: "auth"},
				{Name: "adminpanel", Cause: "auth"},
			},
		},
		{
			name:       "success - unknown names ignored",
			candidates: candidates(),
			disabled:   []string{"no-such-module"},
			wantKept:   []string{"users", "auth", "comments", "adminpanel", "media"},
		},
		{
			name:       "success - duplicate disabled entries collapse",
			candidates: candidates(),
			disabled:   []string{"media", "media"},
			wantKept:   []string{"users", "auth", "comments", "adminpanel"},
			wantDropped: []kernel.DroppedModule{
				{Name: "media", Direct: true, Cause: "media"},
			},
		},
		{
			name: "success - absent but enabled dep is kept for registry fail-fast",
			candidates: []kernel.Module{
				newStubModule("needy", []string{"ghost"}, nil),
			},
			disabled: nil,
			wantKept: []string{"needy"},
		},
		{
			name: "success - disabled but absent dep cascades",
			candidates: []kernel.Module{
				newStubModule("needy", []string{"ghost"}, nil),
			},
			disabled: []string{"ghost"},
			wantDropped: []kernel.DroppedModule{
				{Name: "needy", Cause: "ghost"},
			},
		},
		{
			name: "success - nil candidates skipped",
			candidates: []kernel.Module{
				nil,
				newStubModule("media", nil, nil),
			},
			disabled: nil,
			wantKept: []string{"media"},
		},
		{
			name:       "success - empty candidates",
			candidates: nil,
			disabled:   []string{"users"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := kernel.Prune(tt.candidates, tt.disabled)

			kept := make([]string, 0, len(result.Kept))
			for _, module := range result.Kept {
				require.NotNil(t, module)
				kept = append(kept, module.Name())
			}
			if len(tt.wantKept) == 0 {
				assert.Empty(t, kept)
			} else {
				assert.Equal(t, tt.wantKept, kept)
			}
			if len(tt.wantDropped) == 0 {
				assert.Empty(t, result.Dropped)
			} else {
				assert.Equal(t, tt.wantDropped, result.Dropped)
			}
		})
	}
}
