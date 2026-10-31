package customfield_test

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/kernel/customfield"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeValue_Multiselect(t *testing.T) {
	field := customfield.FieldSchema{
		Name:    "Sizes",
		Slug:    "sizes",
		Type:    customfield.FieldTypeMultiselect,
		Options: []string{"S", "M", "L"},
	}

	tests := []struct {
		name     string
		value    any
		expected any
		wantErr  error
	}{
		{
			name:     "canonical array passes through",
			value:    []any{"S", "L"},
			expected: []any{"S", "L"},
		},
		{
			name:     "legacy comma-separated string coerces",
			value:    "S,L",
			expected: []any{"S", "L"},
		},
		{
			name:    "item outside options rejected",
			value:   []any{"S", "XL"},
			wantErr: customfield.ErrItemNotAllowed,
		},
		{
			name:    "non-string item rejected",
			value:   []any{"S", 42.0},
			wantErr: customfield.ErrItemNotString,
		},
		{
			name:    "non-array non-string value rejected",
			value:   42.0,
			wantErr: customfield.ErrItemNotString,
		},
		{
			name:    "max_items exceeded",
			value:   []any{"S", "M", "L"},
			wantErr: customfield.ErrItemLimitExceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := field
			if tt.name == "max_items exceeded" {
				f.MaxItems = new(2)
			}

			got, err := customfield.NormalizeValue(f, tt.value)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestNormalizeValue_List(t *testing.T) {
	field := customfield.FieldSchema{
		Name: "Specs",
		Slug: "specs",
		Type: customfield.FieldTypeList,
	}

	tests := []struct {
		name     string
		field    customfield.FieldSchema
		value    any
		expected any
		wantErr  error
		errText  string
	}{
		{
			name:     "canonical array passes through",
			field:    field,
			value:    []any{"Cotton", "Machine wash"},
			expected: []any{"Cotton", "Machine wash"},
		},
		{
			name:     "legacy newline-separated string coerces, blank lines dropped",
			field:    field,
			value:    "Cotton\n\nMachine wash\n",
			expected: []any{"Cotton", "Machine wash"},
		},
		{
			name:    "empty item rejected",
			field:   field,
			value:   []any{"Cotton", "  "},
			wantErr: customfield.ErrItemEmpty,
		},
		{
			name:    "non-string item rejected",
			field:   field,
			value:   []any{true},
			wantErr: customfield.ErrItemNotString,
		},
		{
			name:    "per-item max_length exceeded",
			field:   customfield.FieldSchema{Name: "Specs", Slug: "specs", Type: customfield.FieldTypeList, MaxLength: new(5)},
			value:   []any{"Cotton", "Way too long"},
			wantErr: nil,
			errText: "items must be at most 5 characters",
		},
		{
			name:    "default item cap applies",
			field:   field,
			value:   manyStrings(customfield.DefaultMaxItemsList + 1),
			wantErr: customfield.ErrItemLimitExceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := customfield.NormalizeValue(tt.field, tt.value)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			if tt.errText != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errText)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestNormalizeValue_Gallery(t *testing.T) {
	field := customfield.FieldSchema{
		Name: "Images",
		Slug: "images",
		Type: customfield.FieldTypeGallery,
	}

	tests := []struct {
		name     string
		value    any
		expected any
		wantErr  error
	}{
		{
			name: "canonical array passes through",
			value: []any{
				map[string]any{"url": "/uploads/media/abc.webp", "alt": "Front"},
				map[string]any{"url": "https://cdn.example.com/x.webp", "alt": "Back"},
			},
			expected: []any{
				map[string]any{"url": "/uploads/media/abc.webp", "alt": "Front"},
				map[string]any{"url": "https://cdn.example.com/x.webp", "alt": "Back"},
			},
		},
		{
			name:  "legacy one-URL-per-line string coerces with empty alt",
			value: "/uploads/media/abc.webp\nhttps://cdn.example.com/x.webp",
			expected: []any{
				map[string]any{"url": "/uploads/media/abc.webp", "alt": ""},
				map[string]any{"url": "https://cdn.example.com/x.webp", "alt": ""},
			},
		},
		{
			name:    "item missing url rejected",
			value:   []any{map[string]any{"alt": "no url"}},
			wantErr: customfield.ErrGalleryItemInvalid,
		},
		{
			name:    "item with unknown key rejected",
			value:   []any{map[string]any{"url": "/a.webp", "caption": "x"}},
			wantErr: customfield.ErrGalleryItemInvalid,
		},
		{
			name:    "non-object item rejected",
			value:   []any{"/a.webp"},
			wantErr: customfield.ErrGalleryItemInvalid,
		},
		{
			name:    "protocol-relative url rejected",
			value:   []any{map[string]any{"url": "//evil.example.com/a.webp"}},
			wantErr: customfield.ErrGalleryURLInvalid,
		},
		{
			name:    "non-http scheme rejected",
			value:   []any{map[string]any{"url": "javascript:alert(1)"}},
			wantErr: customfield.ErrGalleryURLInvalid,
		},
		{
			name:    "default item cap applies",
			value:   manyGalleryItems(customfield.DefaultMaxItemsGallery + 1),
			wantErr: customfield.ErrItemLimitExceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := customfield.NormalizeValue(field, tt.value)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestNormalizeValue_ScalarTypesPassThrough(t *testing.T) {
	for _, f := range []customfield.FieldSchema{
		{Name: "Note", Slug: "note", Type: customfield.FieldTypeText},
		{Name: "Count", Slug: "count", Type: customfield.FieldTypeNumber},
		{Name: "Live", Slug: "live", Type: customfield.FieldTypeCheckbox},
	} {
		value := any("anything")
		if f.Type == customfield.FieldTypeCheckbox {
			value = true
		}
		if f.Type == customfield.FieldTypeNumber {
			value = 42.0
		}

		got, err := customfield.NormalizeValue(f, value)
		require.NoError(t, err)
		assert.Equal(t, value, got)
	}
}

func manyStrings(n int) []any {
	items := make([]any, 0, n)
	for range n {
		items = append(items, "item")
	}
	return items
}

func manyGalleryItems(n int) []any {
	items := make([]any, 0, n)
	for range n {
		items = append(items, map[string]any{"url": "/uploads/media/x.webp"})
	}
	return items
}
