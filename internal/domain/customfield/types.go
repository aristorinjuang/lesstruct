package customfield

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	ErrFieldNameRequired    = errors.New("field name is required and must be between 1 and 200 characters")
	ErrFieldSlugInvalid     = errors.New("field slug must be between 1 and 200 characters and contain only lowercase letters, numbers, and underscores")
	ErrFieldTypeInvalid     = errors.New("field type must be one of: text, textarea, number, date, select, checkbox")
	ErrSelectRequiresOpts   = errors.New("select fields require an options list")
	ErrSelectEmptyOption    = errors.New("select field options must not contain empty or blank strings")
	ErrNumberMinGTMax       = errors.New("min cannot be greater than max")
	ErrMaxLengthInvalid     = errors.New("max_length must be greater than 0 and is only valid for text and textarea fields")
	ErrDuplicateFieldSlug   = errors.New("duplicate field slug")
)

var slugPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

type FieldType string

const (
	FieldTypeText     FieldType = "text"
	FieldTypeTextarea FieldType = "textarea"
	FieldTypeNumber   FieldType = "number"
	FieldTypeDate     FieldType = "date"
	FieldTypeSelect   FieldType = "select"
	FieldTypeCheckbox FieldType = "checkbox"
)

var validFieldTypes = map[FieldType]bool{
	FieldTypeText:     true,
	FieldTypeTextarea: true,
	FieldTypeNumber:   true,
	FieldTypeDate:     true,
	FieldTypeSelect:   true,
	FieldTypeCheckbox: true,
}

type FieldSchema struct {
	Name      string    `json:"name" toml:"name"`
	Slug      string    `json:"slug" toml:"slug"`
	Type      FieldType `json:"type" toml:"type"`
	Required  bool      `json:"required,omitempty" toml:"required,omitempty"`
	Options   []string  `json:"options,omitempty" toml:"options,omitempty"`
	Min       *float64  `json:"min,omitempty" toml:"min,omitempty"`
	Max       *float64  `json:"max,omitempty" toml:"max,omitempty"`
	MaxLength *int      `json:"maxLength,omitempty" toml:"max_length,omitempty"`
}

func ValidateField(f FieldSchema) error {
	if err := ValidateFieldName(f.Name); err != nil {
		return err
	}

	if err := ValidateFieldSlug(f.Slug); err != nil {
		return err
	}

	if err := ValidateFieldType(f.Type); err != nil {
		return err
	}

	if err := ValidateSelectOptions(f.Type, f.Options); err != nil {
		return err
	}

	if err := ValidateNumberRange(f.Type, f.Min, f.Max); err != nil {
		return err
	}

	if err := ValidateMaxLength(f.Type, f.MaxLength); err != nil {
		return err
	}

	return nil
}

func ValidateFields(fields []FieldSchema) error {
	seen := make(map[string]bool, len(fields))
	for _, f := range fields {
		if err := ValidateField(f); err != nil {
			return fmt.Errorf("field %q: %w", f.Slug, err)
		}

		if seen[f.Slug] {
			return fmt.Errorf("field %q: %w", f.Slug, ErrDuplicateFieldSlug)
		}
		seen[f.Slug] = true
	}
	return nil
}

func ValidateFieldName(name string) error {
	trimmed := strings.TrimSpace(name)
	if utf8.RuneCountInString(trimmed) == 0 || utf8.RuneCountInString(trimmed) > 200 {
		return ErrFieldNameRequired
	}
	return nil
}

func ValidateFieldSlug(slug string) error {
	if utf8.RuneCountInString(slug) == 0 || utf8.RuneCountInString(slug) > 200 {
		return ErrFieldSlugInvalid
	}

	if !slugPattern.MatchString(slug) {
		return ErrFieldSlugInvalid
	}

	return nil
}

func ValidateFieldType(t FieldType) error {
	if !validFieldTypes[t] {
		return ErrFieldTypeInvalid
	}
	return nil
}

func ValidateSelectOptions(t FieldType, options []string) error {
	if t == FieldTypeSelect && len(options) == 0 {
		return ErrSelectRequiresOpts
	}

	if t == FieldTypeSelect {
		for _, opt := range options {
			if strings.TrimSpace(opt) == "" {
				return ErrSelectEmptyOption
			}
		}
	}

	return nil
}

func ValidateNumberRange(t FieldType, min, max *float64) error {
	if t != FieldTypeNumber {
		return nil
	}

	if min != nil && max != nil && *min > *max {
		return ErrNumberMinGTMax
	}
	return nil
}

func ValidateMaxLength(t FieldType, maxLength *int) error {
	if maxLength == nil {
		return nil
	}

	if t != FieldTypeText && t != FieldTypeTextarea {
		return ErrMaxLengthInvalid
	}

	if *maxLength <= 0 {
		return ErrMaxLengthInvalid
	}
	return nil
}
