package posttype_test

import (
	"errors"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/domain/customfield"
	"github.com/aristorinjuang/lesstruct/internal/domain/posttype"
	"github.com/stretchr/testify/assert"
)

func TestPostTypeValidation(t *testing.T) {
	tests := []struct {
		name    string
		pt      posttype.PostType
		wantErr error
	}{
		{
			name: "valid post type",
			pt: posttype.PostType{
				Name:     "Portfolio",
				Slug:     "portfolio",
				Supports: []string{"title", "content", "tags"},
			},
			wantErr: nil,
		},
		{
			name: "empty name",
			pt: posttype.PostType{
				Name:     "",
				Slug:     "portfolio",
				Supports: []string{"title", "content", "tags"},
			},
			wantErr: posttype.ErrInvalidPostTypeName,
		},
		{
			name: "name too long",
			pt: posttype.PostType{
				Name:     string(make([]byte, 201)), // > 200 chars
				Slug:     "portfolio",
				Supports: []string{"title", "content", "tags"},
			},
			wantErr: posttype.ErrInvalidPostTypeName,
		},
		{
			name: "empty slug",
			pt: posttype.PostType{
				Name:     "Portfolio",
				Slug:     "",
				Supports: []string{"title", "content", "tags"},
			},
			wantErr: posttype.ErrInvalidPostTypeSlug,
		},
		{
			name: "slug too long",
			pt: posttype.PostType{
				Name:     "Portfolio",
				Slug:     string(make([]byte, 201)), // > 200 chars
				Supports: []string{"title", "content", "tags"},
			},
			wantErr: posttype.ErrInvalidPostTypeSlug,
		},
		{
			name: "slug with invalid characters - uppercase",
			pt: posttype.PostType{
				Name:     "Portfolio",
				Slug:     "Portfolio",
				Supports: []string{"title", "content", "tags"},
			},
			wantErr: posttype.ErrInvalidPostTypeSlug,
		},
		{
			name: "slug with invalid characters - spaces",
			pt: posttype.PostType{
				Name:     "Portfolio",
				Slug:     "portfolio items",
				Supports: []string{"title", "content", "tags"},
			},
			wantErr: posttype.ErrInvalidPostTypeSlug,
		},
		{
			name: "slug with invalid characters - special chars",
			pt: posttype.PostType{
				Name:     "Portfolio",
				Slug:     "portfolio@items",
				Supports: []string{"title", "content", "tags"},
			},
			wantErr: posttype.ErrInvalidPostTypeSlug,
		},
		{
			name: "slug with underscore - valid",
			pt: posttype.PostType{
				Name:     "Portfolio",
				Slug:     "portfolio_items",
				Supports: []string{"title", "content", "tags"},
			},
			wantErr: nil,
		},
		{
			name: "slug starting with number - valid",
			pt: posttype.PostType{
				Name:     "3D Model",
				Slug:     "3d-model",
				Supports: []string{"title", "content", "tags"},
			},
			wantErr: nil,
		},
		{
			name: "empty supports",
			pt: posttype.PostType{
				Name:     "Portfolio",
				Slug:     "portfolio",
				Supports: []string{},
			},
			wantErr: posttype.ErrInvalidSupports,
		},
		{
			name: "invalid support feature",
			pt: posttype.PostType{
				Name:     "Portfolio",
				Slug:     "portfolio",
				Supports: []string{"title", "invalid_feature"},
			},
			wantErr: posttype.ErrInvalidSupports,
		},
		{
			name: "valid supports - all features",
			pt: posttype.PostType{
				Name:     "Portfolio",
				Slug:     "portfolio",
				Supports: []string{"title", "content", "tags", "featured_image", "excerpt"},
			},
			wantErr: nil,
		},
		{
			name: "valid supports - partial features",
			pt: posttype.PostType{
				Name:     "Media",
				Slug:     "media",
				Supports: []string{"title", "featured_image"},
			},
			wantErr: nil,
		},
		{
			name: "duplicate supports - should be valid",
			pt: posttype.PostType{
				Name:     "Portfolio",
				Slug:     "portfolio",
				Supports: []string{"title", "title", "content"},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := posttype.Validate(tt.pt)
			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("Validate() expected error %v, got nil", tt.wantErr)
				} else if err.Error() != tt.wantErr.Error() {
					t.Errorf("Validate() error = %v, want %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("Validate() unexpected error = %v", err)
			}
		})
	}
}

func TestValidateSlug(t *testing.T) {
	tests := []struct {
		name    string
		slug    string
		wantErr error
	}{
		{"valid simple slug", "portfolio", nil},
		{"valid slug with hyphens", "portfolio-item", nil},
		{"valid slug with numbers", "portfolio-2024", nil},
		{"valid slug with underscores", "portfolio_item", nil},
		{"valid mixed", "portfolio_item-2024", nil},
		{"empty slug", "", posttype.ErrInvalidPostTypeSlug},
		{"slug with spaces", "portfolio item", posttype.ErrInvalidPostTypeSlug},
		{"slug with uppercase", "Portfolio", posttype.ErrInvalidPostTypeSlug},
		{"slug with special chars", "portfolio@item", posttype.ErrInvalidPostTypeSlug},
		{"slug too long", string(make([]byte, 201)), posttype.ErrInvalidPostTypeSlug},
		{"slug starting with hyphen", "-portfolio", posttype.ErrInvalidPostTypeSlug},
		{"slug ending with hyphen", "portfolio-", posttype.ErrInvalidPostTypeSlug},
		{"slug with consecutive hyphens", "portfolio--item", posttype.ErrInvalidPostTypeSlug},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := posttype.ValidateSlug(tt.slug)
			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("ValidateSlug() expected error %v, got nil", tt.wantErr)
				} else if err.Error() != tt.wantErr.Error() {
					t.Errorf("ValidateSlug() error = %v, want %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("ValidateSlug() unexpected error = %v", err)
			}
		})
	}
}

func TestValidateSupports(t *testing.T) {
	tests := []struct {
		name    string
		supports []string
		wantErr error
	}{
		{"valid single support", []string{"title"}, nil},
		{"valid multiple supports", []string{"title", "content", "tags"}, nil},
		{"valid all supports", []string{"title", "content", "tags", "featured_image", "excerpt"}, nil},
		{"empty supports", []string{}, posttype.ErrInvalidSupports},
		{"invalid support", []string{"title", "invalid"}, posttype.ErrInvalidSupports},
		{"all invalid", []string{"invalid1", "invalid2"}, posttype.ErrInvalidSupports},
		{"nil supports", nil, posttype.ErrInvalidSupports},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := posttype.ValidateSupports(tt.supports)
			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("ValidateSupports() expected error %v, got nil", tt.wantErr)
				} else if err.Error() != tt.wantErr.Error() {
					t.Errorf("ValidateSupports() error = %v, want %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("ValidateSupports() unexpected error = %v", err)
			}
		})
	}
}

func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		nameVal string
		wantErr error
	}{
		{"valid name", "Portfolio", nil},
		{"valid name with spaces", "Portfolio Item", nil},
		{"valid name with numbers", "Portfolio 2024", nil},
		{"valid name with special chars", "Portfolio & Design", nil},
		{"empty name", "", posttype.ErrInvalidPostTypeName},
		{"whitespace only name", "   ", posttype.ErrInvalidPostTypeName},
		{"name too long", string(make([]byte, 201)), posttype.ErrInvalidPostTypeName},
		{"name at limit", string(make([]byte, 200)), nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := posttype.ValidateName(tt.nameVal)
			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("ValidateName() expected error %v, got nil", tt.wantErr)
				} else if err.Error() != tt.wantErr.Error() {
					t.Errorf("ValidateName() error = %v, want %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("ValidateName() unexpected error = %v", err)
			}
		})
	}
}

func TestPostTypeValidationWithSystemFields(t *testing.T) {
	tests := []struct {
		name    string
		pt      posttype.PostType
		wantErr error
	}{
		{
			name: "valid post type with valid system fields",
			pt: posttype.PostType{
				Name:     "Product",
				Slug:     "product",
				Supports: []string{"title", "content"},
				SystemFields: []customfield.FieldSchema{
					{Name: "Internal SKU", Slug: "internal_sku", Type: customfield.FieldTypeText},
					{Name: "Sync Status", Slug: "sync_status", Type: customfield.FieldTypeSelect, Options: []string{"pending", "synced", "failed"}},
				},
			},
			wantErr: nil,
		},
		{
			name: "valid post type with both fields and system fields",
			pt: posttype.PostType{
				Name:     "Product",
				Slug:     "product",
				Supports: []string{"title", "content"},
				Fields: []customfield.FieldSchema{
					{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber},
				},
				SystemFields: []customfield.FieldSchema{
					{Name: "Internal SKU", Slug: "internal_sku", Type: customfield.FieldTypeText},
				},
			},
			wantErr: nil,
		},
		{
			name: "post type with invalid system field type",
			pt: posttype.PostType{
				Name:     "Test",
				Slug:     "test",
				Supports: []string{"title"},
				SystemFields: []customfield.FieldSchema{
					{Name: "Bad", Slug: "bad", Type: "invalid"},
				},
			},
			wantErr: customfield.ErrFieldTypeInvalid,
		},
		{
			name: "post type with duplicate system field slugs",
			pt: posttype.PostType{
				Name:     "Test",
				Slug:     "test",
				Supports: []string{"title"},
				SystemFields: []customfield.FieldSchema{
					{Name: "One", Slug: "dup", Type: customfield.FieldTypeText},
					{Name: "Two", Slug: "dup", Type: customfield.FieldTypeText},
				},
			},
			wantErr: customfield.ErrDuplicateFieldSlug,
		},
		{
			name: "valid post type without system fields",
			pt: posttype.PostType{
				Name:     "Portfolio",
				Slug:     "portfolio",
				Supports: []string{"title", "content"},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := posttype.Validate(tt.pt)
			if tt.wantErr != nil {
				assert.True(t, errors.Is(err, tt.wantErr), "Validate() error = %v, want %v", err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSupportedFeatures(t *testing.T) {
	features := posttype.GetSupportedFeatures()

	expectedFeatures := []string{
		"title", "content", "tags", "featured_image", "excerpt",
	}

	if len(features) != len(expectedFeatures) {
		t.Errorf("GetSupportedFeatures() returned %d features, expected %d", len(features), len(expectedFeatures))
	}

	featureMap := make(map[string]bool)
	for _, f := range features {
		featureMap[f] = true
	}

	for _, expected := range expectedFeatures {
		if !featureMap[expected] {
			t.Errorf("GetSupportedFeatures() missing feature: %s", expected)
		}
	}
}

func TestPostTypeValidationWithFields(t *testing.T) {
	tests := []struct {
		name    string
		pt      posttype.PostType
		wantErr error
	}{
		{
			name: "valid post type with valid fields",
			pt: posttype.PostType{
				Name:     "Menu Item",
				Slug:     "menu-item",
				Supports: []string{"title", "content"},
				Fields: []customfield.FieldSchema{
					{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber},
					{Name: "Category", Slug: "category", Type: customfield.FieldTypeSelect, Options: []string{"A", "B"}},
				},
			},
			wantErr: nil,
		},
		{
			name: "valid post type without fields",
			pt: posttype.PostType{
				Name:     "Portfolio",
				Slug:     "portfolio",
				Supports: []string{"title", "content"},
			},
			wantErr: nil,
		},
		{
			name: "post type with invalid field type",
			pt: posttype.PostType{
				Name:     "Test",
				Slug:     "test",
				Supports: []string{"title"},
				Fields: []customfield.FieldSchema{
					{Name: "Bad", Slug: "bad", Type: "invalid"},
				},
			},
			wantErr: customfield.ErrFieldTypeInvalid,
		},
		{
			name: "post type with duplicate field slugs",
			pt: posttype.PostType{
				Name:     "Test",
				Slug:     "test",
				Supports: []string{"title"},
				Fields: []customfield.FieldSchema{
					{Name: "One", Slug: "dup", Type: customfield.FieldTypeText},
					{Name: "Two", Slug: "dup", Type: customfield.FieldTypeText},
				},
			},
			wantErr: customfield.ErrDuplicateFieldSlug,
		},
		{
			name: "post type with select field missing options",
			pt: posttype.PostType{
				Name:     "Test",
				Slug:     "test",
				Supports: []string{"title"},
				Fields: []customfield.FieldSchema{
					{Name: "Cat", Slug: "cat", Type: customfield.FieldTypeSelect},
				},
			},
			wantErr: customfield.ErrSelectRequiresOpts,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := posttype.Validate(tt.pt)
			if tt.wantErr != nil {
				assert.True(t, errors.Is(err, tt.wantErr), "Validate() error = %v, want %v", err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUserFieldsValidation(t *testing.T) {
	tests := []struct {
		name       string
		userFields posttype.UserFields
		wantErr    bool
	}{
		{
			name: "valid user fields with both custom and system",
			userFields: posttype.UserFields{
				Fields: []customfield.FieldSchema{
					{Name: "Job Title", Slug: "job_title", Type: customfield.FieldTypeText},
					{Name: "Company", Slug: "company", Type: customfield.FieldTypeText},
				},
				SystemFields: []customfield.FieldSchema{
					{Name: "Internal Rating", Slug: "internal_rating", Type: customfield.FieldTypeSelect, Options: []string{"bronze", "silver"}},
				},
			},
			wantErr: false,
		},
		{
			name: "valid user fields with custom only",
			userFields: posttype.UserFields{
				Fields: []customfield.FieldSchema{
					{Name: "Website", Slug: "website", Type: customfield.FieldTypeText},
				},
			},
			wantErr: false,
		},
		{
			name:       "empty user fields",
			userFields: posttype.UserFields{},
			wantErr:    false,
		},
		{
			name: "invalid custom field type",
			userFields: posttype.UserFields{
				Fields: []customfield.FieldSchema{
					{Name: "Bad", Slug: "bad", Type: "invalid"},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid system field type",
			userFields: posttype.UserFields{
				SystemFields: []customfield.FieldSchema{
					{Name: "Bad", Slug: "bad", Type: "invalid"},
				},
			},
			wantErr: true,
		},
		{
			name: "duplicate slugs in custom fields",
			userFields: posttype.UserFields{
				Fields: []customfield.FieldSchema{
					{Name: "One", Slug: "dup", Type: customfield.FieldTypeText},
					{Name: "Two", Slug: "dup", Type: customfield.FieldTypeText},
				},
			},
			wantErr: true,
		},
		{
			name: "duplicate slugs in system fields",
			userFields: posttype.UserFields{
				SystemFields: []customfield.FieldSchema{
					{Name: "One", Slug: "dup", Type: customfield.FieldTypeText},
					{Name: "Two", Slug: "dup", Type: customfield.FieldTypeText},
				},
			},
			wantErr: true,
		},
		{
			name: "select custom field without options",
			userFields: posttype.UserFields{
				Fields: []customfield.FieldSchema{
					{Name: "Cat", Slug: "cat", Type: customfield.FieldTypeSelect},
				},
			},
			wantErr: true,
		},
		{
			name: "number custom field min > max",
			userFields: posttype.UserFields{
				Fields: []customfield.FieldSchema{
					{Name: "Score", Slug: "score", Type: customfield.FieldTypeNumber, Min: floatPtr(100), Max: floatPtr(10)},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.userFields.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func floatPtr(v float64) *float64 {
	return &v
}
