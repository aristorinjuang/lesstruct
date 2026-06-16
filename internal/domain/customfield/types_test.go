package customfield_test

import (
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/domain/customfield"
	"github.com/stretchr/testify/assert"
)

func TestValidateField(t *testing.T) {
	minVal := 1.0
	maxVal := 20.0
	maxLen := 500

	tests := []struct {
		name    string
		field   customfield.FieldSchema
		wantErr error
	}{
		{
			name: "valid text field",
			field: customfield.FieldSchema{
				Name: "Title",
				Slug: "title",
				Type: customfield.FieldTypeText,
			},
			wantErr: nil,
		},
		{
			name: "valid textarea field with max_length",
			field: customfield.FieldSchema{
				Name:      "Description",
				Slug:      "description",
				Type:      customfield.FieldTypeTextarea,
				MaxLength: &maxLen,
			},
			wantErr: nil,
		},
		{
			name: "valid number field with min and max",
			field: customfield.FieldSchema{
				Name: "Servings",
				Slug: "servings",
				Type: customfield.FieldTypeNumber,
				Min:  &minVal,
				Max:  &maxVal,
			},
			wantErr: nil,
		},
		{
			name: "valid date field",
			field: customfield.FieldSchema{
				Name: "Available From",
				Slug: "available_from",
				Type: customfield.FieldTypeDate,
			},
			wantErr: nil,
		},
		{
			name: "valid select field with options",
			field: customfield.FieldSchema{
				Name:    "Category",
				Slug:    "category",
				Type:    customfield.FieldTypeSelect,
				Options: []string{"Pastry", "Bread", "Cake"},
			},
			wantErr: nil,
		},
		{
			name: "valid checkbox field",
			field: customfield.FieldSchema{
				Name: "Available",
				Slug: "available",
				Type: customfield.FieldTypeCheckbox,
			},
			wantErr: nil,
		},
		{
			name: "empty name",
			field: customfield.FieldSchema{
				Name: "",
				Slug: "title",
				Type: customfield.FieldTypeText,
			},
			wantErr: customfield.ErrFieldNameRequired,
		},
		{
			name: "name too long",
			field: customfield.FieldSchema{
				Name: string(make([]byte, 201)),
				Slug: "title",
				Type: customfield.FieldTypeText,
			},
			wantErr: customfield.ErrFieldNameRequired,
		},
		{
			name: "empty slug",
			field: customfield.FieldSchema{
				Name: "Title",
				Slug: "",
				Type: customfield.FieldTypeText,
			},
			wantErr: customfield.ErrFieldSlugInvalid,
		},
		{
			name: "slug with hyphens",
			field: customfield.FieldSchema{
				Name: "Title",
				Slug: "my-title",
				Type: customfield.FieldTypeText,
			},
			wantErr: customfield.ErrFieldSlugInvalid,
		},
		{
			name: "slug with uppercase",
			field: customfield.FieldSchema{
				Name: "Title",
				Slug: "Title",
				Type: customfield.FieldTypeText,
			},
			wantErr: customfield.ErrFieldSlugInvalid,
		},
		{
			name: "invalid field type",
			field: customfield.FieldSchema{
				Name: "Title",
				Slug: "title",
				Type: "invalid",
			},
			wantErr: customfield.ErrFieldTypeInvalid,
		},
		{
			name: "select without options",
			field: customfield.FieldSchema{
				Name: "Category",
				Slug: "category",
				Type: customfield.FieldTypeSelect,
			},
			wantErr: customfield.ErrSelectRequiresOpts,
		},
		{
			name: "select with empty options",
			field: customfield.FieldSchema{
				Name:    "Category",
				Slug:    "category",
				Type:    customfield.FieldTypeSelect,
				Options: []string{},
			},
			wantErr: customfield.ErrSelectRequiresOpts,
		},
		{
			name: "number min greater than max",
			field: customfield.FieldSchema{
				Name: "Price",
				Slug: "price",
				Type: customfield.FieldTypeNumber,
				Min:  &maxVal,
				Max:  &minVal,
			},
			wantErr: customfield.ErrNumberMinGTMax,
		},
		{
			name: "number min equals max is valid",
			field: customfield.FieldSchema{
				Name: "Exact",
				Slug: "exact",
				Type: customfield.FieldTypeNumber,
				Min:  &minVal,
				Max:  &minVal,
			},
			wantErr: nil,
		},
		{
			name: "slug too long",
			field: customfield.FieldSchema{
				Name: "Title",
				Slug: string(make([]byte, 201)),
				Type: customfield.FieldTypeText,
			},
			wantErr: customfield.ErrFieldSlugInvalid,
		},
		{
			name: "max_length on non-text type",
			field: customfield.FieldSchema{
				Name:      "Price",
				Slug:      "price",
				Type:      customfield.FieldTypeNumber,
				MaxLength: &maxLen,
			},
			wantErr: customfield.ErrMaxLengthInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := customfield.ValidateField(tt.field)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateFields(t *testing.T) {
	minVal := 1.0
	maxVal := 20.0

	tests := []struct {
		name    string
		fields  []customfield.FieldSchema
		wantErr error
	}{
		{
			name: "valid fields",
			fields: []customfield.FieldSchema{
				{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber, Min: &minVal, Max: &maxVal},
				{Name: "Description", Slug: "description", Type: customfield.FieldTypeTextarea},
			},
			wantErr: nil,
		},
		{
			name:    "empty fields is valid",
			fields:  []customfield.FieldSchema{},
			wantErr: nil,
		},
		{
			name:    "nil fields is valid",
			fields:  nil,
			wantErr: nil,
		},
		{
			name: "duplicate slug",
			fields: []customfield.FieldSchema{
				{Name: "Title", Slug: "title", Type: customfield.FieldTypeText},
				{Name: "Another Title", Slug: "title", Type: customfield.FieldTypeText},
			},
			wantErr: customfield.ErrDuplicateFieldSlug,
		},
		{
			name: "invalid field within list",
			fields: []customfield.FieldSchema{
				{Name: "Valid", Slug: "valid", Type: customfield.FieldTypeText},
				{Name: "Bad", Slug: "bad", Type: "invalid"},
			},
			wantErr: customfield.ErrFieldTypeInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := customfield.ValidateFields(tt.fields)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateFieldName(t *testing.T) {
	tests := []struct {
		name    string
		nameVal string
		wantErr error
	}{
		{"valid name", "Price", nil},
		{"name with spaces", "Available From", nil},
		{"empty name", "", customfield.ErrFieldNameRequired},
		{"name too long", string(make([]byte, 201)), customfield.ErrFieldNameRequired},
		{"name at limit", string(make([]byte, 200)), nil},
		{"whitespace-only name", "   ", customfield.ErrFieldNameRequired},
		{"name with leading trailing spaces", "  Price  ", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := customfield.ValidateFieldName(tt.nameVal)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateFieldSlug(t *testing.T) {
	tests := []struct {
		name    string
		slug    string
		wantErr error
	}{
		{"valid slug", "price", nil},
		{"valid slug with underscores", "available_from", nil},
		{"valid slug with numbers", "field1", nil},
		{"empty slug", "", customfield.ErrFieldSlugInvalid},
		{"slug with hyphens", "my-field", customfield.ErrFieldSlugInvalid},
		{"slug with uppercase", "MyField", customfield.ErrFieldSlugInvalid},
		{"slug starting with number", "1field", customfield.ErrFieldSlugInvalid},
		{"slug too long", string(make([]byte, 201)), customfield.ErrFieldSlugInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := customfield.ValidateFieldSlug(tt.slug)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateFieldType(t *testing.T) {
	tests := []struct {
		name    string
		ftype   customfield.FieldType
		wantErr error
	}{
		{"text type", customfield.FieldTypeText, nil},
		{"textarea type", customfield.FieldTypeTextarea, nil},
		{"number type", customfield.FieldTypeNumber, nil},
		{"date type", customfield.FieldTypeDate, nil},
		{"select type", customfield.FieldTypeSelect, nil},
		{"checkbox type", customfield.FieldTypeCheckbox, nil},
		{"invalid type", customfield.FieldType("invalid"), customfield.ErrFieldTypeInvalid},
		{"empty type", customfield.FieldType(""), customfield.ErrFieldTypeInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := customfield.ValidateFieldType(tt.ftype)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateSelectOptions(t *testing.T) {
	tests := []struct {
		name    string
		ftype   customfield.FieldType
		options []string
		wantErr error
	}{
		{"select with options", customfield.FieldTypeSelect, []string{"A", "B"}, nil},
		{"select without options", customfield.FieldTypeSelect, nil, customfield.ErrSelectRequiresOpts},
		{"select with empty options", customfield.FieldTypeSelect, []string{}, customfield.ErrSelectRequiresOpts},
		{"select with empty string option", customfield.FieldTypeSelect, []string{"A", ""}, customfield.ErrSelectEmptyOption},
		{"select with whitespace-only option", customfield.FieldTypeSelect, []string{"A", "  "}, customfield.ErrSelectEmptyOption},
		{"text type ignores options", customfield.FieldTypeText, nil, nil},
		{"number type ignores options", customfield.FieldTypeNumber, nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := customfield.ValidateSelectOptions(tt.ftype, tt.options)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateNumberRange(t *testing.T) {
	min := 1.0
	max := 20.0
	same := 5.0

	tests := []struct {
		name    string
		ftype   customfield.FieldType
		min     *float64
		max     *float64
		wantErr error
	}{
		{"number min less than max", customfield.FieldTypeNumber, &min, &max, nil},
		{"number min equals max", customfield.FieldTypeNumber, &same, &same, nil},
		{"number min greater than max", customfield.FieldTypeNumber, &max, &min, customfield.ErrNumberMinGTMax},
		{"number only min set", customfield.FieldTypeNumber, &min, nil, nil},
		{"number only max set", customfield.FieldTypeNumber, nil, &max, nil},
		{"number neither set", customfield.FieldTypeNumber, nil, nil, nil},
		{"text type ignores range", customfield.FieldTypeText, &max, &min, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := customfield.ValidateNumberRange(tt.ftype, tt.min, tt.max)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateMaxLength(t *testing.T) {
	validLen := 500
	zeroLen := 0
	negLen := -1

	tests := []struct {
		name      string
		ftype     customfield.FieldType
		maxLength *int
		wantErr   error
	}{
		{"text with max_length", customfield.FieldTypeText, &validLen, nil},
		{"textarea with max_length", customfield.FieldTypeTextarea, &validLen, nil},
		{"nil max_length is valid", customfield.FieldTypeText, nil, nil},
		{"number with max_length rejected", customfield.FieldTypeNumber, &validLen, customfield.ErrMaxLengthInvalid},
		{"date with max_length rejected", customfield.FieldTypeDate, &validLen, customfield.ErrMaxLengthInvalid},
		{"select with max_length rejected", customfield.FieldTypeSelect, &validLen, customfield.ErrMaxLengthInvalid},
		{"checkbox with max_length rejected", customfield.FieldTypeCheckbox, &validLen, customfield.ErrMaxLengthInvalid},
		{"zero max_length rejected", customfield.FieldTypeText, &zeroLen, customfield.ErrMaxLengthInvalid},
		{"negative max_length rejected", customfield.FieldTypeText, &negLen, customfield.ErrMaxLengthInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := customfield.ValidateMaxLength(tt.ftype, tt.maxLength)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
