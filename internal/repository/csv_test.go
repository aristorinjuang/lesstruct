package repository_test

import (
	"encoding/json"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitCSV(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "empty string", input: "", want: []string{}},
		{name: "single value", input: "article", want: []string{"article"}},
		{name: "multiple values", input: "article,post,event", want: []string{"article", "post", "event"}},
		{name: "trailing comma", input: "a,b,", want: []string{"a", "b"}},
		{name: "blank entries dropped", input: "a,,b", want: []string{"a", "b"}},
		{name: "leading comma", input: ",a,b", want: []string{"a", "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := repository.SplitCSV(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestSplitCSV_NeverNil guards the JSON-serialisation contract: even for blank
// input the result must be a non-nil slice so it renders as [] not null.
func TestSplitCSV_NeverNil(t *testing.T) {
	got := repository.SplitCSV("")
	require.NotNil(t, got)
	assert.Len(t, got, 0)
}

func TestMarshalCustomFields(t *testing.T) {
	t.Run("nil map returns nil value (SQL NULL)", func(t *testing.T) {
		v, err := repository.MarshalCustomFields(nil)
		require.NoError(t, err)
		assert.Nil(t, v)
	})

	t.Run("empty map marshals to a JSON object string", func(t *testing.T) {
		v, err := repository.MarshalCustomFields(map[string]any{})
		require.NoError(t, err)
		s, ok := v.(string)
		require.True(t, ok)
		assert.Equal(t, "{}", s)
	})

	t.Run("populated map round-trips through JSON", func(t *testing.T) {
		original := map[string]any{"price": float64(9.99), "category": "Pastry"}
		v, err := repository.MarshalCustomFields(original)
		require.NoError(t, err)
		s, ok := v.(string)
		require.True(t, ok)

		var roundTripped map[string]any
		require.NoError(t, json.Unmarshal([]byte(s), &roundTripped))
		assert.Equal(t, original["price"], roundTripped["price"])
		assert.Equal(t, original["category"], roundTripped["category"])
	})
}
