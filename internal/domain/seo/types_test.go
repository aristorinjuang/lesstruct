package seo_test

import (
	"strings"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/domain/seo"
)

func TestValidateMetaDescription(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{
			name:    "valid meta description",
			input:   "This is a valid meta description",
			wantErr: nil,
		},
		{
			name:    "empty meta description",
			input:   "",
			wantErr: seo.ErrInvalidMetaDescription,
		},
		{
			name:    "whitespace only meta description",
			input:   "   ",
			wantErr: seo.ErrInvalidMetaDescription,
		},
		{
			name:    "meta description with leading/trailing whitespace",
			input:   "  valid meta description  ",
			wantErr: nil,
		},
		{
			name:    "meta description exactly 160 characters",
			input:   strings.Repeat("a", 160),
			wantErr: nil,
		},
		{
			name:    "meta description over 160 characters",
			input:   strings.Repeat("a", 161),
			wantErr: seo.ErrInvalidMetaDescription,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := seo.ValidateMetaDescription(tt.input)
			if err != tt.wantErr {
				t.Errorf("ValidateMetaDescription() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateOGTitle(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{
			name:    "valid OG title",
			input:   "Valid OG Title",
			wantErr: nil,
		},
		{
			name:    "empty OG title",
			input:   "",
			wantErr: seo.ErrInvalidOGTitle,
		},
		{
			name:    "whitespace only OG title",
			input:   "   ",
			wantErr: seo.ErrInvalidOGTitle,
		},
		{
			name:    "OG title with leading/trailing whitespace",
			input:   "  Valid OG Title  ",
			wantErr: nil,
		},
		{
			name:    "OG title exactly 60 characters",
			input:   strings.Repeat("a", 60),
			wantErr: nil,
		},
		{
			name:    "OG title over 60 characters",
			input:   strings.Repeat("a", 61),
			wantErr: seo.ErrInvalidOGTitle,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := seo.ValidateOGTitle(tt.input)
			if err != tt.wantErr {
				t.Errorf("ValidateOGTitle() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateOGDescription(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{
			name:    "valid OG description",
			input:   "Valid OG description",
			wantErr: nil,
		},
		{
			name:    "empty OG description",
			input:   "",
			wantErr: seo.ErrInvalidOGDescription,
		},
		{
			name:    "whitespace only OG description",
			input:   "   ",
			wantErr: seo.ErrInvalidOGDescription,
		},
		{
			name:    "OG description with leading/trailing whitespace",
			input:   "  Valid OG description  ",
			wantErr: nil,
		},
		{
			name:    "OG description exactly 160 characters",
			input:   strings.Repeat("a", 160),
			wantErr: nil,
		},
		{
			name:    "OG description over 160 characters",
			input:   strings.Repeat("a", 161),
			wantErr: seo.ErrInvalidOGDescription,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := seo.ValidateOGDescription(tt.input)
			if err != tt.wantErr {
				t.Errorf("ValidateOGDescription() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
