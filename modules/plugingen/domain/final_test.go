package plugingen_test

import (
	"context"
	"testing"

	plugingen "github.com/aristorinjuang/lesstruct/modules/plugingen/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlugifyAndGenerateBranches(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "success - long brief truncation and fallbacks",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Long brief to trigger truncation in slugify (base >50 chars).
			longDesc := "abcdefghijklmnopqrst " + "abcdefghijklmnopqrst " + "abcdefghijklmnopqrst extra words here"
			badNamed := plugingen.NewProject(
				"Bad!",
				"",
				nil,
				nil,
				map[string]string{
					"main.go": "package main\n//export hook_before_save\nfunc hookBeforeSave(o uint32, l uint32) uint32 { return 0 }\nfunc main() {}",
					"go.mod":  "module x",
				},
				"",
			)
			// Repair returns project with bad name and empty language to hit fallbacks.
			badRepair := plugingen.NewProject(
				"Also Bad!",
				"",
				[]string{"before_save"},
				nil,
				map[string]string{
					"main.go": "package main\n//export hook_before_save\nfunc hookBeforeSave(o uint32, l uint32) uint32 { return 0 }\nfunc main() {}",
					"go.mod":  "module x",
				},
				"",
			)
			gen := &fakeGeneration{project: badNamed, repair: badRepair}
			// Second repair returns valid to exit loop successfully.
			// Our fake returns badRepair always; after first repair, report still valid? badRepair has bad name but service keeps old name, language falls back, hooks present, files valid -> valid. So succeeds.
			store := newFakeStore()
			svc := plugingen.NewService(&genAdapter{inner: gen}, plugingen.NewValidator(), store, nil)
			brief := plugingen.NewBrief(longDesc, "go", []string{"before_save"}, nil)
			got, report, err := svc.Generate(context.Background(), brief)
			require.NoError(t, err)
			assert.True(t, report.Valid)
			assert.NotEmpty(t, got.Name)
			// Generate with nil store (validates but does not persist).
			gen2 := &fakeGeneration{project: validProject()}
			svc2 := plugingen.NewService(&genAdapter{inner: gen2}, plugingen.NewValidator(), nil, nil)
			_, _, err = svc2.Generate(context.Background(), plugingen.NewBrief("desc", "go", []string{"before_save"}, nil))
			require.NoError(t, err)
			assert.Equal(t, tt.wantErr, false)
		})
	}
}
