package posttype_test

import (
	"strings"
	"testing"

	posttype "github.com/aristorinjuang/lesstruct/modules/posttypes-basic/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateNameID(t *testing.T) {
	tests := []struct {
		name    string
		nameID  string
		wantErr bool
	}{
		{
			name:    "success - empty allowed",
			nameID:  "",
			wantErr: false,
		},
		{
			name:    "success - valid Indonesian name",
			nameID:  "Produk",
			wantErr: false,
		},
		{
			name:    "success - whitespace only treated as empty",
			nameID:  "   ",
			wantErr: false,
		},
		{
			name:    "error - too long",
			nameID:  strings.Repeat("a", 201),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := posttype.ValidateNameID(tt.nameID)
			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, posttype.ErrInvalidPostTypeNameID)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestValidateDescriptionID(t *testing.T) {
	tests := []struct {
		name    string
		descID  string
		wantErr bool
	}{
		{
			name:    "success - empty allowed",
			descID:  "",
			wantErr: false,
		},
		{
			name:    "success - valid Indonesian description",
			descID:  "Deskripsi produk",
			wantErr: false,
		},
		{
			name:    "error - too long",
			descID:  strings.Repeat("a", 501),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := posttype.ValidateDescriptionID(tt.descID)
			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, posttype.ErrInvalidPostTypeDescriptionID)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestValidate_Localized(t *testing.T) {
	tests := []struct {
		name    string
		pt      posttype.PostType
		wantErr bool
	}{
		{
			name: "success - valid with Indonesian overrides",
			pt: posttype.PostType{
				Name:          "Product",
				Slug:          "product",
				Description:   "Base",
				NameID:        "Produk",
				DescriptionID: "Deskripsi",
				Supports:      []string{"title"},
			},
			wantErr: false,
		},
		{
			name: "success - empty overrides allowed",
			pt: posttype.PostType{
				Name:     "Product",
				Slug:     "product",
				Supports: []string{"title"},
			},
			wantErr: false,
		},
		{
			name: "error - invalid name_id",
			pt: posttype.PostType{
				Name:     "Product",
				Slug:     "product",
				NameID:   strings.Repeat("a", 201),
				Supports: []string{"title"},
			},
			wantErr: true,
		},
		{
			name: "error - invalid description_id",
			pt: posttype.PostType{
				Name:          "Product",
				Slug:          "product",
				DescriptionID: strings.Repeat("a", 501),
				Supports:      []string{"title"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := posttype.Validate(tt.pt)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestService_RegisterLocalizedExtension(t *testing.T) {
	tests := []struct {
		name       string
		ext        posttype.PostType
		wantErr    bool
		wantNameID string
		wantDescID string
	}{
		{
			name: "success - extension merges localized overrides",
			ext: posttype.PostType{
				Slug:          "post",
				NameID:        "Artikel",
				DescriptionID: "Deskripsi ID",
			},
			wantErr:    false,
			wantNameID: "Artikel",
			wantDescID: "Deskripsi ID",
		},
		{
			name: "success - empty overrides preserve existing values",
			ext: posttype.PostType{
				Slug: "post",
			},
			wantErr:    false,
			wantNameID: "",
			wantDescID: "",
		},
		{
			name: "error - extension with too-long name_id rejected",
			ext: posttype.PostType{
				Slug:   "post",
				NameID: strings.Repeat("a", 201),
			},
			wantErr: true,
		},
		{
			name: "error - extension with too-long description_id rejected",
			ext: posttype.PostType{
				Slug:          "post",
				DescriptionID: strings.Repeat("a", 501),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := posttype.NewService()
			err := svc.Register(tt.ext)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			got, err := svc.GetBySlug("post")
			require.NoError(t, err)
			assert.Equal(t, tt.wantNameID, got.NameID)
			assert.Equal(t, tt.wantDescID, got.DescriptionID)
		})
	}
}
