package posttype_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/aristorinjuang/lesstruct/internal/domain/customfield"
	"github.com/aristorinjuang/lesstruct/internal/domain/posttype"
)

func TestGetDefaultPostTypes(t *testing.T) {
	defaultTypes := posttype.GetDefaultPostTypes()

	if len(defaultTypes) != 4 {
		t.Errorf("GetDefaultPostTypes() returned %d types, expected 4", len(defaultTypes))
	}

	slugMap := make(map[string]bool)
	for _, pt := range defaultTypes {
		slugMap[pt.Slug] = true
	}

	expectedSlugs := []string{"post", "page", "media", "comment"}
	for _, slug := range expectedSlugs {
		if !slugMap[slug] {
			t.Errorf("GetDefaultPostTypes() missing slug: %s", slug)
		}
	}

	// Validate each default post type
	for _, pt := range defaultTypes {
		if err := posttype.Validate(pt); err != nil {
			t.Errorf("Default post type %s is invalid: %v", pt.Slug, err)
		}
	}
}

func TestService_RegisterAndGetBySlug(t *testing.T) {
	service := posttype.NewService()

	pt := posttype.PostType{
		Name:     "Portfolio",
		Slug:     "portfolio",
		Supports: []string{"title", "content", "tags"},
	}

	// Register a post type
	err := service.Register(pt)
	if err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	// Get the registered post type
	retrieved, err := service.GetBySlug("portfolio")
	if err != nil {
		t.Fatalf("GetBySlug() failed: %v", err)
	}

	if retrieved.Name != pt.Name {
		t.Errorf("GetBySlug() Name = %s, want %s", retrieved.Name, pt.Name)
	}
	if retrieved.Slug != pt.Slug {
		t.Errorf("GetBySlug() Slug = %s, want %s", retrieved.Slug, pt.Slug)
	}

	// Test getting non-existent post type
	_, err = service.GetBySlug("nonexistent")
	if err != posttype.ErrPostTypeNotFound {
		t.Errorf("GetBySlug() error = %v, want %v", err, posttype.ErrPostTypeNotFound)
	}
}

func TestService_RegisterDuplicate(t *testing.T) {
	service := posttype.NewService()

	pt := posttype.PostType{
		Name:     "Portfolio",
		Slug:     "portfolio",
		Supports: []string{"title", "content", "tags"},
	}

	// Register first time
	err := service.Register(pt)
	if err != nil {
		t.Fatalf("First Register() failed: %v", err)
	}

	// Try to register duplicate
	err = service.Register(pt)
	if err != posttype.ErrDuplicatePostType {
		t.Errorf("Second Register() error = %v, want %v", err, posttype.ErrDuplicatePostType)
	}
}

func TestService_RegisterExtendsDefault(t *testing.T) {
	service := posttype.NewService()

	// Registering a built-in slug EXTENDS it: the field merges in while the
	// built-in identity (Name/Description/Supports) is preserved. (Replaces the
	// old reject-on-default-slug behavior.)
	pt := posttype.PostType{
		Slug: "post", // built-in slug
		Fields: []customfield.FieldSchema{
			{Name: "Mood", Slug: "mood", Type: customfield.FieldTypeText},
		},
	}

	if err := service.Register(pt); err != nil {
		t.Fatalf("Register() extending default slug error = %v, want nil", err)
	}

	// Built-in identity preserved.
	defaultPost, err := service.GetBySlug("post")
	if err != nil {
		t.Fatalf("GetBySlug(post) failed: %v", err)
	}
	if defaultPost.Name != "Post" {
		t.Errorf("Default post identity changed: Name = %s, want 'Post'", defaultPost.Name)
	}
	if defaultPost.Description != "Blog posts and articles" {
		t.Errorf("Default post description changed: %s", defaultPost.Description)
	}
	wantSupports := []string{"title", "content", "tags", "featured_image"}
	if len(defaultPost.Supports) != len(wantSupports) {
		t.Errorf("Default post supports changed: %v, want %v", defaultPost.Supports, wantSupports)
	}

	// The merged field is visible via GetFieldsByPostType (content validation
	// depends on this).
	fields, err := service.GetFieldsByPostType("post")
	if err != nil {
		t.Fatalf("GetFieldsByPostType(post) failed: %v", err)
	}
	if len(fields) != 1 || fields[0].Slug != "mood" {
		t.Errorf("Merged field missing: got %+v, want one field with slug 'mood'", fields)
	}
}

func TestService_RegisterMergeDefault(t *testing.T) {
	textField := func(slug string) customfield.FieldSchema {
		return customfield.FieldSchema{Name: slug, Slug: slug, Type: customfield.FieldTypeText}
	}

	t.Run("appends new field", func(t *testing.T) {
		s := posttype.NewService()
		if err := s.Register(posttype.PostType{Slug: "post", Fields: []customfield.FieldSchema{textField("mood")}}); err != nil {
			t.Fatalf("Register: %v", err)
		}
		fields, _ := s.GetFieldsByPostType("post")
		if len(fields) != 1 || fields[0].Slug != "mood" {
			t.Errorf("appended field missing: %+v", fields)
		}
	})

	t.Run("replaces existing field by slug", func(t *testing.T) {
		s := posttype.NewService()
		if err := s.Register(posttype.PostType{Slug: "post", Fields: []customfield.FieldSchema{
			{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber},
		}}); err != nil {
			t.Fatalf("seed Register: %v", err)
		}
		// A second extension with the same slug but a different type replaces it.
		if err := s.Register(posttype.PostType{Slug: "post", Fields: []customfield.FieldSchema{
			{Name: "Price", Slug: "price", Type: customfield.FieldTypeText},
		}}); err != nil {
			t.Fatalf("replace Register: %v", err)
		}
		fields, _ := s.GetFieldsByPostType("post")
		if len(fields) != 1 {
			t.Fatalf("expected a single replaced field, got %+v", fields)
		}
		if fields[0].Type != customfield.FieldTypeText {
			t.Errorf("field not replaced: type = %v, want text", fields[0].Type)
		}
	})

	t.Run("merges system fields", func(t *testing.T) {
		s := posttype.NewService()
		if err := s.Register(posttype.PostType{Slug: "post", SystemFields: []customfield.FieldSchema{textField("internal_id")}}); err != nil {
			t.Fatalf("Register: %v", err)
		}
		sys, _ := s.GetSystemFieldsByPostType("post")
		if len(sys) != 1 || sys[0].Slug != "internal_id" {
			t.Errorf("merged system field missing: %+v", sys)
		}
	})

	t.Run("ignores incoming name and supports", func(t *testing.T) {
		s := posttype.NewService()
		if err := s.Register(posttype.PostType{
			Name:     "Ignored",
			Slug:     "post",
			Supports: []string{"excerpt"},
			Fields:   []customfield.FieldSchema{textField("mood")},
		}); err != nil {
			t.Fatalf("Register: %v", err)
		}
		pt, _ := s.GetBySlug("post")
		if pt.Name != "Post" {
			t.Errorf("incoming Name was applied: %s", pt.Name)
		}
		want := []string{"title", "content", "tags", "featured_image"}
		if len(pt.Supports) != len(want) {
			t.Errorf("incoming Supports were applied: %v, want %v", pt.Supports, want)
		}
	})

	t.Run("no fields is a no-op success", func(t *testing.T) {
		s := posttype.NewService()
		if err := s.Register(posttype.PostType{Slug: "post"}); err != nil {
			t.Fatalf("Register with no fields: %v", err)
		}
		fields, _ := s.GetFieldsByPostType("post")
		if len(fields) != 0 {
			t.Errorf("expected no fields, got %+v", fields)
		}
	})

	t.Run("rejects invalid incoming field", func(t *testing.T) {
		s := posttype.NewService()
		err := s.Register(posttype.PostType{Slug: "post", Fields: []customfield.FieldSchema{
			{Name: "Bad", Slug: "bad", Type: customfield.FieldType("nope")},
		}})
		if err == nil {
			t.Errorf("expected error for invalid field type, got nil")
		}
	})

	t.Run("rejects invalid incoming system field", func(t *testing.T) {
		s := posttype.NewService()
		err := s.Register(posttype.PostType{Slug: "post", SystemFields: []customfield.FieldSchema{
			{Name: "Bad", Slug: "bad", Type: customfield.FieldType("nope")},
		}})
		if err == nil {
			t.Errorf("expected error for invalid system field type, got nil")
		}
	})

	t.Run("non-default duplicate slug still errors", func(t *testing.T) {
		s := posttype.NewService()
		pt := posttype.PostType{Name: "Portfolio", Slug: "portfolio", Supports: []string{"title"}}
		if err := s.Register(pt); err != nil {
			t.Fatalf("first Register: %v", err)
		}
		err := s.Register(pt)
		if !errors.Is(err, posttype.ErrDuplicatePostType) {
			t.Errorf("duplicate non-default slug error = %v, want ErrDuplicatePostType", err)
		}
	})
}

func TestService_RegisterInvalid(t *testing.T) {
	service := posttype.NewService()

	pt := posttype.PostType{
		Name:     "", // Invalid
		Slug:     "portfolio",
		Supports: []string{"title", "content"},
	}

	err := service.Register(pt)
	if err == nil {
		t.Errorf("Register() with invalid post type expected error, got nil")
	}
}

func TestService_GetAll(t *testing.T) {
	service := posttype.NewService()

	// Initially, should have default post types
	allTypes := service.GetAll()
	if len(allTypes) != 4 {
		t.Errorf("GetAll() initially returned %d types, expected 4", len(allTypes))
	}

	// Register a custom post type
	pt := posttype.PostType{
		Name:     "Portfolio",
		Slug:     "portfolio",
		Supports: []string{"title", "content", "tags"},
	}
	_ = service.Register(pt)

	// Now should have 5 types
	allTypes = service.GetAll()
	if len(allTypes) != 5 {
		t.Errorf("GetAll() after registration returned %d types, expected 5", len(allTypes))
	}

	// Check that the custom type is included
	found := false
	for _, pt := range allTypes {
		if pt.Slug == "portfolio" {
			found = true
			break
		}
	}
	if !found {
		t.Error("GetAll() did not include the registered custom post type")
	}
}

func TestService_LoadConfigFromFile(t *testing.T) {
	// Create a temporary directory for test files
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "post-types.toml")

	// Write a test TOML config
	tomlContent := `# Test custom post types
[[post_type]]
name = "Portfolio"
slug = "portfolio"
description = "Portfolio items"
supports = ["title", "content", "tags", "featured_image"]

[[post_type]]
name = "Product"
slug = "product"
description = "Product listings"
supports = ["title", "content", "featured_image"]
`
	err := os.WriteFile(configFile, []byte(tomlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Create service and load config
	service := posttype.NewService()
	err = service.LoadConfigFromFile(configFile)
	if err != nil {
		t.Fatalf("LoadConfigFromFile() failed: %v", err)
	}

	// Verify custom post types are loaded
	pt1, err := service.GetBySlug("portfolio")
	if err != nil {
		t.Errorf("GetBySlug(portfolio) failed: %v", err)
	}
	if pt1.Name != "Portfolio" {
		t.Errorf("portfolio Name = %s, want Portfolio", pt1.Name)
	}

	pt2, err := service.GetBySlug("product")
	if err != nil {
		t.Errorf("GetBySlug(product) failed: %v", err)
	}
	if pt2.Name != "Product" {
		t.Errorf("product Name = %s, want Product", pt2.Name)
	}
}

func TestService_LoadConfigFromNonExistentFile(t *testing.T) {
	// Create service with non-existent config path
	service := posttype.NewService()
	err := service.LoadConfigFromFile("/nonexistent/path/post-types.toml")
	if err != nil {
		t.Errorf("LoadConfigFromFile() with non-existent file should use defaults, got error: %v", err)
	}

	// Should still have default post types
	allTypes := service.GetAll()
	if len(allTypes) != 4 {
		t.Errorf("GetAll() after failed load returned %d types, expected 4", len(allTypes))
	}
}

func TestService_LoadConfigInvalidTOML(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "invalid.toml")

	// Write invalid TOML
	err := os.WriteFile(configFile, []byte("invalid toml content [["), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	service := posttype.NewService()
	err = service.LoadConfigFromFile(configFile)
	if err == nil {
		t.Errorf("LoadConfigFromFile() with invalid TOML expected error, got nil")
	}
}

func TestService_LoadConfigWithInvalidPostType(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "invalid-post-type.toml")

	// Write TOML with invalid post type
	tomlContent := `[[post_type]]
name = ""
slug = "invalid"
supports = ["title"]
`
	err := os.WriteFile(configFile, []byte(tomlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	service := posttype.NewService()
	err = service.LoadConfigFromFile(configFile)
	if err == nil {
		t.Errorf("LoadConfigFromFile() with invalid post type expected error, got nil")
	}
}

func TestService_LoadConfigWithDuplicateSlugs(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "duplicate-slugs.toml")

	// Write TOML with duplicate slugs
	tomlContent := `[[post_type]]
name = "First"
slug = "duplicate"
supports = ["title"]

[[post_type]]
name = "Second"
slug = "duplicate"
supports = ["content"]
`
	err := os.WriteFile(configFile, []byte(tomlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	service := posttype.NewService()
	err = service.LoadConfigFromFile(configFile)
	if err == nil {
		t.Errorf("LoadConfigFromFile() with duplicate slugs expected error, got nil")
	}
}

func TestService_GetUserSystemFieldSlugs(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "user-system-fields.toml")
	tomlContent := `[user_fields]
[[user_fields.system_fields]]
name = "Internal"
slug = "internal"
type = "text"
`
	if err := os.WriteFile(configFile, []byte(tomlContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	service := posttype.NewService()
	if err := service.LoadConfigFromFile(configFile); err != nil {
		t.Fatalf("LoadConfigFromFile: %v", err)
	}
	slugs := service.GetUserSystemFieldSlugs()
	if len(slugs) != 1 || slugs[0] != "internal" {
		t.Errorf("GetUserSystemFieldSlugs() = %v, want [internal]", slugs)
	}
}

func TestService_LoadConfigExtendsDefault(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "extend-default.toml")

	// A [[post_type]] with a built-in slug extends it: only fields/system_fields
	// are read; name/description/supports are ignored and may be omitted.
	tomlContent := `[[post_type]]
slug = "post"

[[post_type.fields]]
name = "Mood"
slug = "mood"
type = "text"
`
	err := os.WriteFile(configFile, []byte(tomlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	service := posttype.NewService()
	if err := service.LoadConfigFromFile(configFile); err != nil {
		t.Fatalf("LoadConfigFromFile() extending default slug error = %v, want nil", err)
	}

	// Built-in identity preserved.
	defaultPost, err := service.GetBySlug("post")
	if err != nil {
		t.Fatalf("GetBySlug(post) failed: %v", err)
	}
	if defaultPost.Name != "Post" {
		t.Errorf("Default post identity changed: Name = %s, want 'Post'", defaultPost.Name)
	}
	if defaultPost.Description != "Blog posts and articles" {
		t.Errorf("Default post description changed: %s", defaultPost.Description)
	}

	// The configured field merged in.
	fields, err := service.GetFieldsByPostType("post")
	if err != nil {
		t.Fatalf("GetFieldsByPostType(post) failed: %v", err)
	}
	if len(fields) != 1 || fields[0].Slug != "mood" {
		t.Errorf("Merged field missing: got %+v, want one field with slug 'mood'", fields)
	}
}

func TestService_LoadConfigMergesDefaultFieldsCumulatively(t *testing.T) {
	// Two [[post_type]] blocks with the same built-in slug cumulatively merge
	// their fields (last-wins on slug collisions), instead of erroring.
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "cumulative-extend.toml")

	tomlContent := `[[post_type]]
slug = "post"

[[post_type.fields]]
name = "Mood"
slug = "mood"
type = "text"

[[post_type]]
slug = "post"

[[post_type.fields]]
name = "Topic"
slug = "topic"
type = "text"
`
	if err := os.WriteFile(configFile, []byte(tomlContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	service := posttype.NewService()
	if err := service.LoadConfigFromFile(configFile); err != nil {
		t.Fatalf("LoadConfigFromFile() cumulative extend error = %v, want nil", err)
	}

	fields, err := service.GetFieldsByPostType("post")
	if err != nil {
		t.Fatalf("GetFieldsByPostType(post) failed: %v", err)
	}
	gotSlugs := make(map[string]bool, len(fields))
	for _, f := range fields {
		gotSlugs[f.Slug] = true
	}
	if !gotSlugs["mood"] || !gotSlugs["topic"] {
		t.Errorf("cumulative merge missing fields: got %+v, want both mood and topic", fields)
	}
}

func TestService_InitializeWithDefaults(t *testing.T) {
	service := posttype.NewService()

	// Service should be initialized with default post types
	allTypes := service.GetAll()
	if len(allTypes) != 4 {
		t.Errorf("NewService() GetAll() returned %d types, expected 4", len(allTypes))
	}

	// Verify we can get each default type
	defaultSlugs := []string{"post", "page", "media", "comment"}
	for _, slug := range defaultSlugs {
		_, err := service.GetBySlug(slug)
		if err != nil {
			t.Errorf("GetBySlug(%s) failed: %v", slug, err)
		}
	}
}

func TestService_IsSupported(t *testing.T) {
	service := posttype.NewService()

	// Test with default post types
	pt, _ := service.GetBySlug("post")

	// "post" supports title, content, tags, featured_image
	if !service.IsSupported(pt, "title") {
		t.Error("IsSupported() returned false for 'title' which should be supported")
	}
	if !service.IsSupported(pt, "content") {
		t.Error("IsSupported() returned false for 'content' which should be supported")
	}
	if !service.IsSupported(pt, "tags") {
		t.Error("IsSupported() returned false for 'tags' which should be supported")
	}
	if !service.IsSupported(pt, "featured_image") {
		t.Error("IsSupported() returned false for 'featured_image' which should be supported")
	}

	// "post" does not support excerpt
	if service.IsSupported(pt, "excerpt") {
		t.Error("IsSupported() returned true for 'excerpt' which should not be supported")
	}

	// Test invalid feature
	if service.IsSupported(pt, "invalid_feature") {
		t.Error("IsSupported() returned true for invalid feature")
	}
}

func TestService_GetAllBySupports(t *testing.T) {
	service := posttype.NewService()

	// Get post types that support "tags"
	typesWithTags := service.GetAllBySupports("tags")

	// "post" supports tags, "page" and "media" don't (based on default config)
	if len(typesWithTags) == 0 {
		t.Error("GetAllBySupports('tags') returned no results, expected at least 'post'")
	}

	// Verify "post" is in the results
	found := false
	for _, pt := range typesWithTags {
		if pt.Slug == "post" {
			found = true
			break
		}
	}
	if !found {
		t.Error("GetAllBySupports('tags') did not include 'post'")
	}
}

func TestService_LoadConfigFromDirectory(t *testing.T) {
	// Create service and try to load from a directory instead of a file
	// This will cause an error that's not IsNotExist
	tempDir := t.TempDir()
	service := posttype.NewService()
	err := service.LoadConfigFromFile(tempDir)
	if err == nil {
		t.Errorf("LoadConfigFromFile() with directory path expected error, got nil")
	}
}

func TestService_LoadConfigWithFields(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "post-types.toml")

	tomlContent := `[[post_type]]
name = "Menu Item"
slug = "menu-item"
description = "Restaurant menu items"
supports = ["title", "content", "featured_image"]

[[post_type.fields]]
name = "Price"
slug = "price"
type = "number"
min = 0.0
max = 99999.99
required = true

[[post_type.fields]]
name = "Description"
slug = "description"
type = "textarea"
max_length = 500

[[post_type.fields]]
name = "Category"
slug = "category"
type = "select"
options = ["Pastry", "Bread", "Cake", "Drink"]
required = true

[[post_type.fields]]
name = "Available"
slug = "available"
type = "checkbox"
`
	err := os.WriteFile(configFile, []byte(tomlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	service := posttype.NewService()
	err = service.LoadConfigFromFile(configFile)
	if err != nil {
		t.Fatalf("LoadConfigFromFile() with fields failed: %v", err)
	}

	pt, err := service.GetBySlug("menu-item")
	if err != nil {
		t.Fatalf("GetBySlug(menu-item) failed: %v", err)
	}

	if len(pt.Fields) != 4 {
		t.Fatalf("Expected 4 fields, got %d", len(pt.Fields))
	}

	// Verify number field
	if pt.Fields[0].Name != "Price" {
		t.Errorf("Field[0] Name = %s, want Price", pt.Fields[0].Name)
	}
	if pt.Fields[0].Slug != "price" {
		t.Errorf("Field[0] Slug = %s, want price", pt.Fields[0].Slug)
	}
	if pt.Fields[0].Type != "number" {
		t.Errorf("Field[0] Type = %s, want number", pt.Fields[0].Type)
	}
	if pt.Fields[0].Min == nil || *pt.Fields[0].Min != 0.0 {
		t.Errorf("Field[0] Min = %v, want 0.0", pt.Fields[0].Min)
	}
	if pt.Fields[0].Max == nil || *pt.Fields[0].Max != 99999.99 {
		t.Errorf("Field[0] Max = %v, want 99999.99", pt.Fields[0].Max)
	}
	if !pt.Fields[0].Required {
		t.Error("Field[0] Required = false, want true")
	}

	// Verify textarea field with max_length
	if pt.Fields[1].Type != "textarea" {
		t.Errorf("Field[1] Type = %s, want textarea", pt.Fields[1].Type)
	}
	if pt.Fields[1].MaxLength == nil || *pt.Fields[1].MaxLength != 500 {
		t.Errorf("Field[1] MaxLength = %v, want 500", pt.Fields[1].MaxLength)
	}

	// Verify select field with options
	if pt.Fields[2].Type != "select" {
		t.Errorf("Field[2] Type = %s, want select", pt.Fields[2].Type)
	}
	if len(pt.Fields[2].Options) != 4 {
		t.Errorf("Field[2] Options length = %d, want 4", len(pt.Fields[2].Options))
	}
	if pt.Fields[2].Options[0] != "Pastry" {
		t.Errorf("Field[2] Options[0] = %s, want Pastry", pt.Fields[2].Options[0])
	}

	// Verify checkbox field
	if pt.Fields[3].Type != "checkbox" {
		t.Errorf("Field[3] Type = %s, want checkbox", pt.Fields[3].Type)
	}
}

func TestService_LoadConfigWithInvalidFieldType(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "invalid-field-type.toml")

	tomlContent := `[[post_type]]
name = "Test"
slug = "test"
supports = ["title"]

[[post_type.fields]]
name = "Bad"
slug = "bad"
type = "invalid_type"
`
	err := os.WriteFile(configFile, []byte(tomlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	service := posttype.NewService()
	err = service.LoadConfigFromFile(configFile)
	if err == nil {
		t.Error("LoadConfigFromFile() with invalid field type expected error, got nil")
	}
}

func TestService_LoadConfigSelectWithoutOptions(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "select-no-options.toml")

	tomlContent := `[[post_type]]
name = "Test"
slug = "test"
supports = ["title"]

[[post_type.fields]]
name = "Category"
slug = "category"
type = "select"
`
	err := os.WriteFile(configFile, []byte(tomlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	service := posttype.NewService()
	err = service.LoadConfigFromFile(configFile)
	if err == nil {
		t.Error("LoadConfigFromFile() with select without options expected error, got nil")
	}
}

func TestService_LoadConfigNumberMinGTMax(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "min-gt-max.toml")

	tomlContent := `[[post_type]]
name = "Test"
slug = "test"
supports = ["title"]

[[post_type.fields]]
name = "Price"
slug = "price"
type = "number"
min = 100
max = 10
`
	err := os.WriteFile(configFile, []byte(tomlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	service := posttype.NewService()
	err = service.LoadConfigFromFile(configFile)
	if err == nil {
		t.Error("LoadConfigFromFile() with min > max expected error, got nil")
	}
}

func TestService_LoadConfigBackwardCompatibility(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "no-fields.toml")

	// TOML without any fields — should work as before
	tomlContent := `[[post_type]]
name = "Portfolio"
slug = "portfolio"
description = "Portfolio items"
supports = ["title", "content", "tags", "featured_image"]
`
	err := os.WriteFile(configFile, []byte(tomlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	service := posttype.NewService()
	err = service.LoadConfigFromFile(configFile)
	if err != nil {
		t.Fatalf("LoadConfigFromFile() without fields failed: %v", err)
	}

	pt, err := service.GetBySlug("portfolio")
	if err != nil {
		t.Fatalf("GetBySlug(portfolio) failed: %v", err)
	}

	if len(pt.Fields) != 0 {
		t.Errorf("Expected no fields, got %d", len(pt.Fields))
	}
}

func TestService_GetFieldsByPostType(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "post-types.toml")

	tomlContent := `[[post_type]]
name = "Menu Item"
slug = "menu-item"
description = "Restaurant menu items"
supports = ["title", "content"]

[[post_type.fields]]
name = "Price"
slug = "price"
type = "number"
required = true

[[post_type.fields]]
name = "Category"
slug = "category"
type = "select"
options = ["A", "B"]
`
	err := os.WriteFile(configFile, []byte(tomlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	service := posttype.NewService()
	err = service.LoadConfigFromFile(configFile)
	if err != nil {
		t.Fatalf("LoadConfigFromFile() failed: %v", err)
	}

	fields, err := service.GetFieldsByPostType("menu-item")
	if err != nil {
		t.Fatalf("GetFieldsByPostType() failed: %v", err)
	}

	if len(fields) != 2 {
		t.Errorf("Expected 2 fields, got %d", len(fields))
	}
	if fields[0].Slug != "price" {
		t.Errorf("Field[0] Slug = %s, want price", fields[0].Slug)
	}
	if fields[1].Slug != "category" {
		t.Errorf("Field[1] Slug = %s, want category", fields[1].Slug)
	}
}

func TestService_GetFieldsByPostTypeNotFound(t *testing.T) {
	service := posttype.NewService()

	_, err := service.GetFieldsByPostType("nonexistent")
	if err != posttype.ErrPostTypeNotFound {
		t.Errorf("GetFieldsByPostType() error = %v, want %v", err, posttype.ErrPostTypeNotFound)
	}
}

func TestService_GetFieldsByPostTypeNoFields(t *testing.T) {
	service := posttype.NewService()

	fields, err := service.GetFieldsByPostType("post")
	if err != nil {
		t.Fatalf("GetFieldsByPostType(post) failed: %v", err)
	}

	if len(fields) != 0 {
		t.Errorf("Default post type should have no fields, got %d", len(fields))
	}
}

func TestService_LoadConfigDuplicateFieldSlugs(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "dup-field-slugs.toml")

	tomlContent := `[[post_type]]
name = "Test"
slug = "test"
supports = ["title"]

[[post_type.fields]]
name = "Title One"
slug = "title"
type = "text"

[[post_type.fields]]
name = "Title Two"
slug = "title"
type = "text"
`
	err := os.WriteFile(configFile, []byte(tomlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	service := posttype.NewService()
	err = service.LoadConfigFromFile(configFile)
	if err == nil {
		t.Error("LoadConfigFromFile() with duplicate field slugs expected error, got nil")
	}
}

func TestService_RegisterWithFields(t *testing.T) {
	service := posttype.NewService()

	minVal := 0.0
	maxVal := 100.0

	pt := posttype.PostType{
		Name:     "Product",
		Slug:     "product",
		Supports: []string{"title", "content"},
		Fields: []customfield.FieldSchema{
			{Name: "Price", Slug: "price", Type: customfield.FieldTypeNumber, Min: &minVal, Max: &maxVal},
			{Name: "Color", Slug: "color", Type: customfield.FieldTypeSelect, Options: []string{"Red", "Blue"}},
		},
	}

	err := service.Register(pt)
	if err != nil {
		t.Fatalf("Register() with fields failed: %v", err)
	}

	retrieved, err := service.GetBySlug("product")
	if err != nil {
		t.Fatalf("GetBySlug() failed: %v", err)
	}

	if len(retrieved.Fields) != 2 {
		t.Errorf("Expected 2 fields, got %d", len(retrieved.Fields))
	}
}

func TestService_LoadConfigWithSystemFields(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "post-types.toml")

	tomlContent := `[[post_type]]
name = "Product"
slug = "product"
description = "Product listings"
supports = ["title", "content", "featured_image"]

[[post_type.fields]]
name = "Price"
slug = "price"
type = "number"
min = 0.0
max = 99999.99
required = true

[[post_type.system_fields]]
name = "Internal SKU"
slug = "internal_sku"
type = "text"

[[post_type.system_fields]]
name = "Sync Status"
slug = "sync_status"
type = "select"
options = ["pending", "synced", "failed"]
`
	err := os.WriteFile(configFile, []byte(tomlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	service := posttype.NewService()
	err = service.LoadConfigFromFile(configFile)
	if err != nil {
		t.Fatalf("LoadConfigFromFile() with system_fields failed: %v", err)
	}

	pt, err := service.GetBySlug("product")
	if err != nil {
		t.Fatalf("GetBySlug(product) failed: %v", err)
	}

	if len(pt.Fields) != 1 {
		t.Fatalf("Expected 1 custom field, got %d", len(pt.Fields))
	}
	if pt.Fields[0].Slug != "price" {
		t.Errorf("Fields[0] Slug = %s, want price", pt.Fields[0].Slug)
	}

	if len(pt.SystemFields) != 2 {
		t.Fatalf("Expected 2 system fields, got %d", len(pt.SystemFields))
	}
	if pt.SystemFields[0].Slug != "internal_sku" {
		t.Errorf("SystemFields[0] Slug = %s, want internal_sku", pt.SystemFields[0].Slug)
	}
	if pt.SystemFields[1].Slug != "sync_status" {
		t.Errorf("SystemFields[1] Slug = %s, want sync_status", pt.SystemFields[1].Slug)
	}
	if len(pt.SystemFields[1].Options) != 3 {
		t.Errorf("SystemFields[1] Options length = %d, want 3", len(pt.SystemFields[1].Options))
	}
}

func TestService_LoadConfigSystemFieldsOnly(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "post-types.toml")

	tomlContent := `[[post_type]]
name = "Internal Doc"
slug = "internal-doc"
description = "Internal documents"
supports = ["title", "content"]

[[post_type.system_fields]]
name = "Classification"
slug = "classification"
type = "select"
options = ["public", "internal", "confidential"]

[[post_type.system_fields]]
name = "Review Status"
slug = "review_status"
type = "text"
`
	err := os.WriteFile(configFile, []byte(tomlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	service := posttype.NewService()
	err = service.LoadConfigFromFile(configFile)
	if err != nil {
		t.Fatalf("LoadConfigFromFile() with system_fields only failed: %v", err)
	}

	pt, err := service.GetBySlug("internal-doc")
	if err != nil {
		t.Fatalf("GetBySlug(internal-doc) failed: %v", err)
	}

	if len(pt.Fields) != 0 {
		t.Errorf("Expected no custom fields, got %d", len(pt.Fields))
	}
	if len(pt.SystemFields) != 2 {
		t.Errorf("Expected 2 system fields, got %d", len(pt.SystemFields))
	}
}

func TestService_LoadConfigInvalidSystemFieldType(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "invalid-system-field-type.toml")

	tomlContent := `[[post_type]]
name = "Test"
slug = "test"
supports = ["title"]

[[post_type.system_fields]]
name = "Bad"
slug = "bad"
type = "invalid_type"
`
	err := os.WriteFile(configFile, []byte(tomlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	service := posttype.NewService()
	err = service.LoadConfigFromFile(configFile)
	if err == nil {
		t.Error("LoadConfigFromFile() with invalid system field type expected error, got nil")
	}
}

func TestService_LoadConfigSystemFieldSelectWithoutOptions(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "select-no-options.toml")

	tomlContent := `[[post_type]]
name = "Test"
slug = "test"
supports = ["title"]

[[post_type.system_fields]]
name = "Status"
slug = "status"
type = "select"
`
	err := os.WriteFile(configFile, []byte(tomlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	service := posttype.NewService()
	err = service.LoadConfigFromFile(configFile)
	if err == nil {
		t.Error("LoadConfigFromFile() with select system field without options expected error, got nil")
	}
}

func TestService_LoadConfigSystemFieldNumberMinGTMax(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "min-gt-max.toml")

	tomlContent := `[[post_type]]
name = "Test"
slug = "test"
supports = ["title"]

[[post_type.system_fields]]
name = "Priority"
slug = "priority"
type = "number"
min = 100
max = 10
`
	err := os.WriteFile(configFile, []byte(tomlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	service := posttype.NewService()
	err = service.LoadConfigFromFile(configFile)
	if err == nil {
		t.Error("LoadConfigFromFile() with system field min > max expected error, got nil")
	}
}

func TestService_LoadConfigSystemFieldBackwardCompatibility(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "no-system-fields.toml")

	tomlContent := `[[post_type]]
name = "Portfolio"
slug = "portfolio"
description = "Portfolio items"
supports = ["title", "content", "tags", "featured_image"]

[[post_type.fields]]
name = "Price"
slug = "price"
type = "number"
required = true
`
	err := os.WriteFile(configFile, []byte(tomlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	service := posttype.NewService()
	err = service.LoadConfigFromFile(configFile)
	if err != nil {
		t.Fatalf("LoadConfigFromFile() without system_fields failed: %v", err)
	}

	pt, err := service.GetBySlug("portfolio")
	if err != nil {
		t.Fatalf("GetBySlug(portfolio) failed: %v", err)
	}

	if len(pt.Fields) != 1 {
		t.Errorf("Expected 1 custom field, got %d", len(pt.Fields))
	}
	if len(pt.SystemFields) != 0 {
		t.Errorf("Expected no system fields, got %d", len(pt.SystemFields))
	}
}

func TestService_GetSystemFieldsByPostType(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "post-types.toml")

	tomlContent := `[[post_type]]
name = "Product"
slug = "product"
description = "Product listings"
supports = ["title", "content"]

[[post_type.system_fields]]
name = "Internal SKU"
slug = "internal_sku"
type = "text"

[[post_type.system_fields]]
name = "Sync Status"
slug = "sync_status"
type = "select"
options = ["pending", "synced", "failed"]
`
	err := os.WriteFile(configFile, []byte(tomlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	service := posttype.NewService()
	err = service.LoadConfigFromFile(configFile)
	if err != nil {
		t.Fatalf("LoadConfigFromFile() failed: %v", err)
	}

	fields, err := service.GetSystemFieldsByPostType("product")
	if err != nil {
		t.Fatalf("GetSystemFieldsByPostType() failed: %v", err)
	}

	if len(fields) != 2 {
		t.Errorf("Expected 2 system fields, got %d", len(fields))
	}
	if fields[0].Slug != "internal_sku" {
		t.Errorf("SystemField[0] Slug = %s, want internal_sku", fields[0].Slug)
	}
	if fields[1].Slug != "sync_status" {
		t.Errorf("SystemField[1] Slug = %s, want sync_status", fields[1].Slug)
	}
}

func TestService_GetSystemFieldsByPostTypeNotFound(t *testing.T) {
	service := posttype.NewService()

	_, err := service.GetSystemFieldsByPostType("nonexistent")
	if err != posttype.ErrPostTypeNotFound {
		t.Errorf("GetSystemFieldsByPostType() error = %v, want %v", err, posttype.ErrPostTypeNotFound)
	}
}

func TestService_GetSystemFieldsByPostTypeNoSystemFields(t *testing.T) {
	service := posttype.NewService()

	fields, err := service.GetSystemFieldsByPostType("post")
	if err != nil {
		t.Fatalf("GetSystemFieldsByPostType(post) failed: %v", err)
	}

	if len(fields) != 0 {
		t.Errorf("Default post type should have no system fields, got %d", len(fields))
	}
}

func TestService_LoadConfigWithUserFields(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "post-types.toml")

	tomlContent := `[user_fields]
fields = [
  { name = "Job Title", slug = "job_title", type = "text" },
  { name = "Company", slug = "company", type = "text" },
  { name = "Website", slug = "website", type = "text" }
]
system_fields = [
  { name = "Internal Rating", slug = "internal_rating", type = "select", options = ["bronze", "silver", "gold", "platinum"] }
]
`
	err := os.WriteFile(configFile, []byte(tomlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	service := posttype.NewService()
	err = service.LoadConfigFromFile(configFile)
	if err != nil {
		t.Fatalf("LoadConfigFromFile() with user_fields failed: %v", err)
	}

	customFields := service.GetUserFields()
	if len(customFields) != 3 {
		t.Fatalf("Expected 3 custom user fields, got %d", len(customFields))
	}
	if customFields[0].Slug != "job_title" {
		t.Errorf("Fields[0] Slug = %s, want job_title", customFields[0].Slug)
	}

	systemFields := service.GetUserSystemFields()
	if len(systemFields) != 1 {
		t.Fatalf("Expected 1 system user field, got %d", len(systemFields))
	}
	if systemFields[0].Slug != "internal_rating" {
		t.Errorf("SystemFields[0] Slug = %s, want internal_rating", systemFields[0].Slug)
	}
	if len(systemFields[0].Options) != 4 {
		t.Errorf("SystemFields[0] Options length = %d, want 4", len(systemFields[0].Options))
	}
}

func TestService_LoadConfigUserFieldsOnlyCustom(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "post-types.toml")

	tomlContent := `[user_fields]
fields = [
  { name = "Job Title", slug = "job_title", type = "text" }
]
`
	err := os.WriteFile(configFile, []byte(tomlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	service := posttype.NewService()
	err = service.LoadConfigFromFile(configFile)
	if err != nil {
		t.Fatalf("LoadConfigFromFile() with custom-only user_fields failed: %v", err)
	}

	customFields := service.GetUserFields()
	if len(customFields) != 1 {
		t.Fatalf("Expected 1 custom user field, got %d", len(customFields))
	}

	systemFields := service.GetUserSystemFields()
	if len(systemFields) != 0 {
		t.Errorf("Expected no system user fields, got %d", len(systemFields))
	}
}

func TestService_LoadConfigUserFieldsInvalidType(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "invalid-user-field-type.toml")

	tomlContent := `[user_fields]
fields = [
  { name = "Bad", slug = "bad", type = "invalid_type" }
]
`
	err := os.WriteFile(configFile, []byte(tomlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	service := posttype.NewService()
	err = service.LoadConfigFromFile(configFile)
	if err == nil {
		t.Error("LoadConfigFromFile() with invalid user field type expected error, got nil")
	}
}

func TestService_LoadConfigUserFieldsSelectWithoutOptions(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "select-no-options.toml")

	tomlContent := `[user_fields]
fields = [
  { name = "Category", slug = "category", type = "select" }
]
`
	err := os.WriteFile(configFile, []byte(tomlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	service := posttype.NewService()
	err = service.LoadConfigFromFile(configFile)
	if err == nil {
		t.Error("LoadConfigFromFile() with select user field without options expected error, got nil")
	}
}

func TestService_LoadConfigUserFieldsNumberMinGTMax(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "min-gt-max.toml")

	tomlContent := `[user_fields]
fields = [
  { name = "Score", slug = "score", type = "number", min = 100, max = 10 }
]
`
	err := os.WriteFile(configFile, []byte(tomlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	service := posttype.NewService()
	err = service.LoadConfigFromFile(configFile)
	if err == nil {
		t.Error("LoadConfigFromFile() with user field min > max expected error, got nil")
	}
}

func TestService_LoadConfigUserFieldsBackwardCompatibility(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "no-user-fields.toml")

	tomlContent := `[[post_type]]
name = "Portfolio"
slug = "portfolio"
description = "Portfolio items"
supports = ["title", "content"]
`
	err := os.WriteFile(configFile, []byte(tomlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	service := posttype.NewService()
	err = service.LoadConfigFromFile(configFile)
	if err != nil {
		t.Fatalf("LoadConfigFromFile() without user_fields failed: %v", err)
	}

	customFields := service.GetUserFields()
	if len(customFields) != 0 {
		t.Errorf("Expected no user fields, got %d", len(customFields))
	}

	systemFields := service.GetUserSystemFields()
	if len(systemFields) != 0 {
		t.Errorf("Expected no user system fields, got %d", len(systemFields))
	}
}

func TestService_GetUserFields(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "post-types.toml")

	tomlContent := `[user_fields]
fields = [
  { name = "Job Title", slug = "job_title", type = "text" },
  { name = "Company", slug = "company", type = "text" }
]
`
	err := os.WriteFile(configFile, []byte(tomlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	service := posttype.NewService()
	err = service.LoadConfigFromFile(configFile)
	if err != nil {
		t.Fatalf("LoadConfigFromFile() failed: %v", err)
	}

	fields := service.GetUserFields()
	if len(fields) != 2 {
		t.Fatalf("Expected 2 user fields, got %d", len(fields))
	}
	if fields[0].Slug != "job_title" {
		t.Errorf("Fields[0] Slug = %s, want job_title", fields[0].Slug)
	}
	if fields[1].Slug != "company" {
		t.Errorf("Fields[1] Slug = %s, want company", fields[1].Slug)
	}

	// Verify defensive copy — modifying returned slice should not affect service state
	fields[0].Slug = "modified"
	fields2 := service.GetUserFields()
	if fields2[0].Slug != "job_title" {
		t.Error("GetUserFields() did not return a defensive copy")
	}
}

func TestService_GetUserSystemFields(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "post-types.toml")

	tomlContent := `[user_fields]
system_fields = [
  { name = "Internal Rating", slug = "internal_rating", type = "select", options = ["bronze", "silver", "gold"] }
]
`
	err := os.WriteFile(configFile, []byte(tomlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	service := posttype.NewService()
	err = service.LoadConfigFromFile(configFile)
	if err != nil {
		t.Fatalf("LoadConfigFromFile() failed: %v", err)
	}

	fields := service.GetUserSystemFields()
	if len(fields) != 1 {
		t.Fatalf("Expected 1 user system field, got %d", len(fields))
	}
	if fields[0].Slug != "internal_rating" {
		t.Errorf("SystemFields[0] Slug = %s, want internal_rating", fields[0].Slug)
	}

	// Verify defensive copy
	fields[0].Slug = "modified"
	fields2 := service.GetUserSystemFields()
	if fields2[0].Slug != "internal_rating" {
		t.Error("GetUserSystemFields() did not return a defensive copy")
	}
}

func TestService_GetUserFieldsEmpty(t *testing.T) {
	service := posttype.NewService()

	customFields := service.GetUserFields()
	if len(customFields) != 0 {
		t.Errorf("Expected empty user fields from default service, got %d", len(customFields))
	}

	systemFields := service.GetUserSystemFields()
	if len(systemFields) != 0 {
		t.Errorf("Expected empty user system fields from default service, got %d", len(systemFields))
	}
}
