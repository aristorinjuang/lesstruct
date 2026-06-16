package content_test

import (
	"errors"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/domain/content"
	"github.com/aristorinjuang/lesstruct/internal/domain/customfield"
	"github.com/aristorinjuang/lesstruct/internal/domain/posttype"
)

func TestPostTypeAdapter_GetBySlug(t *testing.T) {
	tests := []struct {
		name      string
		slug      string
		setupMock func(*posttype.Service)
		wantErr   error
	}{
		{
			name: "successful retrieval",
			slug: "post",
			setupMock: func(s *posttype.Service) {
				pt := posttype.PostType{
					Name:        "Post",
					Slug:        "post",
					Description: "Blog posts",
					Supports:    []string{"title", "content"},
				}
				_ = s.Register(pt)
			},
			wantErr: nil,
		},
		{
			name: "post type not found",
			slug: "nonexistent",
			setupMock: func(s *posttype.Service) {
			},
			wantErr: posttype.ErrPostTypeNotFound,
		},
		{
			name: "retrieves page post type",
			slug: "page",
			setupMock: func(s *posttype.Service) {
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := posttype.NewService()
			tt.setupMock(service)

			adapter := content.NewPostTypeAdapter(service)

			result, err := adapter.GetBySlug(tt.slug)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("PostTypeAdapter.GetBySlug() expected error, got nil")
					return
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("PostTypeAdapter.GetBySlug() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("PostTypeAdapter.GetBySlug() unexpected error = %v", err)
				return
			}

			if result.Slug != tt.slug {
				t.Errorf("PostTypeAdapter.GetBySlug() Slug = %v, want %v", result.Slug, tt.slug)
			}
		})
	}
}

func TestPostTypeAdapter_GetFieldsByPostType(t *testing.T) {
	tests := []struct {
		name        string
		slug        string
		setupMock   func(*posttype.Service)
		wantErr     error
		wantLen     int
		wantSlugs   []string
	}{
		{
			name: "returns custom fields for post type",
			slug: "product",
			setupMock: func(s *posttype.Service) {
				maxLen := 200
				pt := posttype.PostType{
					Name:     "Product",
					Slug:     "product",
					Supports: []string{"title", "content"},
					Fields: []customfield.FieldSchema{
						{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber, Required: true, Min: ptrF(0), Max: ptrF(10000)},
						{Name: "SKU", Slug: "sku", Type: customfield.FieldTypeText, MaxLength: &maxLen},
					},
				}
				_ = s.Register(pt)
			},
			wantErr:   nil,
			wantLen:   2,
			wantSlugs: []string{"price", "sku"},
		},
		{
			name: "returns empty slice when no custom fields",
			slug: "post",
			setupMock: func(s *posttype.Service) {
			},
			wantErr:   nil,
			wantLen:   0,
			wantSlugs: nil,
		},
		{
			name: "returns error for unknown post type",
			slug: "nonexistent",
			setupMock: func(s *posttype.Service) {
			},
			wantErr:   posttype.ErrPostTypeNotFound,
			wantLen:   0,
			wantSlugs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := posttype.NewService()
			tt.setupMock(service)

			adapter := content.NewPostTypeAdapter(service)

			result, err := adapter.GetFieldsByPostType(tt.slug)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("expected error, got nil")
					return
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error = %v", err)
				return
			}

			if len(result) != tt.wantLen {
				t.Errorf("got %d fields, want %d", len(result), tt.wantLen)
			}

			for i, slug := range tt.wantSlugs {
				if i < len(result) && result[i].Slug != slug {
					t.Errorf("field[%d].Slug = %v, want %v", i, result[i].Slug, slug)
				}
			}
		})
	}
}

func TestPostTypeAdapter_GetSystemFieldsByPostType(t *testing.T) {
	tests := []struct {
		name      string
		slug      string
		setupMock func(*posttype.Service)
		wantErr   error
		wantLen   int
	}{
		{
			name: "returns system fields for post type",
			slug: "product",
			setupMock: func(s *posttype.Service) {
				pt := posttype.PostType{
					Name:     "Product",
					Slug:     "product",
					Supports: []string{"title", "content"},
					SystemFields: []customfield.FieldSchema{
						{Name: "Internal SKU", Slug: "internal_sku", Type: customfield.FieldTypeText},
						{Name: "Sync Status", Slug: "sync_status", Type: customfield.FieldTypeSelect, Options: []string{"pending", "synced", "error"}},
					},
				}
				_ = s.Register(pt)
			},
			wantErr: nil,
			wantLen: 2,
		},
		{
			name: "returns empty slice when no system fields",
			slug: "post",
			setupMock: func(s *posttype.Service) {
			},
			wantErr: nil,
			wantLen: 0,
		},
		{
			name: "returns error for unknown post type",
			slug: "nonexistent",
			setupMock: func(s *posttype.Service) {
			},
			wantErr: posttype.ErrPostTypeNotFound,
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := posttype.NewService()
			tt.setupMock(service)

			adapter := content.NewPostTypeAdapter(service)

			result, err := adapter.GetSystemFieldsByPostType(tt.slug)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("expected error, got nil")
					return
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error = %v", err)
				return
			}

			if len(result) != tt.wantLen {
				t.Errorf("got %d system fields, want %d", len(result), tt.wantLen)
			}
		})
	}
}

func TestNewPostTypeAdapter(t *testing.T) {
	service := posttype.NewService()

	adapter := content.NewPostTypeAdapter(service)

	if adapter == nil {
		t.Errorf("NewPostTypeAdapter() returned nil")
	}
}
