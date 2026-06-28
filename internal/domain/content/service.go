package content

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/aristorinjuang/lesstruct/internal/domain/customfield"
	"github.com/aristorinjuang/lesstruct/internal/domain/plugin"
	"github.com/aristorinjuang/lesstruct/internal/domain/seo"
)

// hookData is the JSON structure sent to and received from plugin hooks.
type hookData struct {
	ContentID    int            `json:"contentId"`
	UserID       int            `json:"userId"`
	Title        string         `json:"title"`
	Content      string         `json:"content"`
	Tags         []string       `json:"tags"`
	Status       string         `json:"status"`
	PostType     string         `json:"postType"`
	CustomFields map[string]any `json:"customFields,omitempty"`
}

func validateFieldValue(
	field customfield.FieldSchema,
	value any,
) error {
	if value == nil {
		return fmt.Errorf("cannot be null")
	}

	switch field.Type {
	case customfield.FieldTypeText, customfield.FieldTypeTextarea:
		str, ok := value.(string)
		if !ok {
			return fmt.Errorf("must be a string")
		}
		if field.MaxLength != nil && utf8.RuneCountInString(str) > *field.MaxLength {
			return fmt.Errorf("must be at most %d characters", *field.MaxLength)
		}

	case customfield.FieldTypeNumber:
		num, ok := value.(float64)
		if !ok {
			return fmt.Errorf("must be a number")
		}
		if field.Min != nil && num < *field.Min {
			return fmt.Errorf("must be at least %g", *field.Min)
		}
		if field.Max != nil && num > *field.Max {
			return fmt.Errorf("must be at most %g", *field.Max)
		}

	case customfield.FieldTypeDate:
		str, ok := value.(string)
		if !ok || strings.TrimSpace(str) == "" {
			return fmt.Errorf("must be a date string")
		}
		if _, err := time.Parse("2006-01-02", str); err != nil {
			return fmt.Errorf("must be a valid date in YYYY-MM-DD format")
		}

	case customfield.FieldTypeSelect:
		str, ok := value.(string)
		if !ok {
			return fmt.Errorf("must be a string")
		}
		if len(field.Options) == 0 {
			return fmt.Errorf("has no configured options")
		}
		if !slices.Contains(field.Options, str) {
			return fmt.Errorf("must be one of: %s", strings.Join(field.Options, ", "))
		}

	case customfield.FieldTypeCheckbox:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("must be a boolean")
		}
	default:
		return fmt.Errorf("unknown field type: %s", field.Type)
	}

	return nil
}

func isEmpty(value any) bool {
	if value == nil {
		return true
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v) == ""
	case float64:
		return v == 0
	}
	return false
}

// CreateCommentRequest represents a request to create a comment
type CreateCommentRequest struct {
	Comment string `json:"comment"`
}

// UpdateCommentStatusRequest represents a request to update comment status
type UpdateCommentStatusRequest struct {
	Status CommentStatus `json:"status"`
}

// CreateContentRequest represents a request to create content
type CreateContentRequest struct {
	Title              string         `json:"title"`
	Content            string         `json:"content"`
	Tags               []string       `json:"tags"`
	Status             Status         `json:"status"`
	PostType           string         `json:"postType"`
	MetaDescription    string         `json:"metaDescription,omitempty"`
	OGTitle            string         `json:"ogTitle,omitempty"`
	OGDescription      string         `json:"ogDescription,omitempty"`
	AllowComments      *bool          `json:"allowComments,omitempty"`
	CustomFields       map[string]any `json:"customFields,omitempty"`
	Language           string         `json:"language,omitempty"`
	TranslationGroupID *int           `json:"translationGroupId,omitempty"`
}

// UpdateContentRequest represents a request to update content
type UpdateContentRequest struct {
	Title              string         `json:"title"`
	Content            string         `json:"content"`
	Tags               []string       `json:"tags"`
	Status             Status         `json:"status"`
	PostType           string         `json:"postType"`
	MetaDescription    string         `json:"metaDescription,omitempty"`
	OGTitle            string         `json:"ogTitle,omitempty"`
	OGDescription      string         `json:"ogDescription,omitempty"`
	AllowComments      *bool          `json:"allowComments,omitempty"`
	CustomFields       map[string]any `json:"customFields,omitempty"`
	Language           string         `json:"language,omitempty"`
	TranslationGroupID *int           `json:"translationGroupId,omitempty"`
}

// HookExecutor executes plugin hooks during content lifecycle operations.
type HookExecutor interface {
	Execute(ctx context.Context, hookName plugin.HookName, data []byte) ([]byte, error)
}

// Service handles content business logic
type Service struct {
	repo            Repository
	commentRepo     CommentRepository
	seoService      *seo.Service
	postTypeService PostTypeServiceInterface
	hookExecutor    HookExecutor
}

func (s *Service) validateCustomFields(
	postTypeSlug string,
	customFields map[string]any,
) error {
	if s.postTypeService == nil {
		return nil
	}

	fields, err := s.postTypeService.GetFieldsByPostType(postTypeSlug)
	if err != nil {
		return err
	}

	for _, field := range fields {
		value, exists := customFields[field.Slug]
		if field.Required && field.Type != customfield.FieldTypeCheckbox && (!exists || isEmpty(value)) {
			return fmt.Errorf("%s is required", field.Name)
		}
		if !exists {
			continue
		}
		if err := validateFieldValue(field, value); err != nil {
			return fmt.Errorf("%s: %w", field.Name, err)
		}
	}
	return nil
}

func (s *Service) stripSystemFields(postTypeSlug string, customFields map[string]any) {
	if s.postTypeService == nil || customFields == nil {
		return
	}
	systemFields, err := s.postTypeService.GetSystemFieldsByPostType(postTypeSlug)
	if err != nil {
		return
	}
	for _, sf := range systemFields {
		delete(customFields, sf.Slug)
	}
}

func (s *Service) getSystemFieldSlugs(postTypeSlug string) []string {
	if s.postTypeService == nil {
		return nil
	}
	systemFields, err := s.postTypeService.GetSystemFieldsByPostType(postTypeSlug)
	if err != nil {
		return nil
	}
	slugs := make([]string, len(systemFields))
	for i, sf := range systemFields {
		slugs[i] = sf.Slug
	}
	return slugs
}

func (s *Service) getSystemFieldValues(postTypeSlug string, customFields map[string]any) map[string]any {
	if len(customFields) == 0 {
		return nil
	}
	slugs := s.getSystemFieldSlugs(postTypeSlug)
	if len(slugs) == 0 {
		return nil
	}
	values := make(map[string]any)
	for _, slug := range slugs {
		if v, ok := customFields[slug]; ok {
			values[slug] = v
		}
	}
	return values
}

func (s *Service) buildHookData(userID int, req CreateContentRequest) []byte {
	data := hookData{
		ContentID:    0,
		UserID:       userID,
		Title:        req.Title,
		Content:      req.Content,
		Tags:         req.Tags,
		Status:       string(req.Status),
		PostType:     req.PostType,
		CustomFields: req.CustomFields,
	}
	b, _ := json.Marshal(data)
	return b
}

func (s *Service) buildHookDataForUpdate(existing *Content, req UpdateContentRequest) []byte {
	merged := make(map[string]any)
	if existing.CustomFields != nil {
		maps.Copy(merged, existing.CustomFields)
	}
	if req.CustomFields != nil {
		maps.Copy(merged, req.CustomFields)
	}
	data := hookData{
		ContentID:    existing.ID,
		UserID:       existing.UserID,
		Title:        req.Title,
		Content:      req.Content,
		Tags:         req.Tags,
		Status:       string(req.Status),
		PostType:     req.PostType,
		CustomFields: merged,
	}
	b, _ := json.Marshal(data)
	return b
}

func (s *Service) applyHookResult(req *CreateContentRequest, result []byte) {
	var parsed hookData
	if err := json.Unmarshal(result, &parsed); err != nil {
		return
	}
	if parsed.CustomFields != nil {
		req.CustomFields = parsed.CustomFields
	}
}

func (s *Service) applyHookResultForUpdate(req *UpdateContentRequest, result []byte) {
	var parsed hookData
	if err := json.Unmarshal(result, &parsed); err != nil {
		return
	}
	if parsed.CustomFields != nil {
		req.CustomFields = parsed.CustomFields
	}
}

func (s *Service) diffSystemFieldValues(
	preHook, postHook map[string]any,
) map[string]any {
	if postHook == nil {
		return nil
	}
	delta := make(map[string]any)
	for key, newVal := range postHook {
		oldVal, existed := preHook[key]
		if !existed {
			delta[key] = newVal
			continue
		}
		if !reflect.DeepEqual(oldVal, newVal) {
			delta[key] = newVal
		}
	}
	return delta
}

func (s *Service) validatePluginSystemFields(
	postTypeSlug string,
	pluginFields map[string]any,
) error {
	if len(pluginFields) == 0 || s.postTypeService == nil {
		return nil
	}
	systemFieldSchemas, err := s.postTypeService.GetSystemFieldsByPostType(postTypeSlug)
	if err != nil {
		return fmt.Errorf("failed to get system field schemas: %w", err)
	}

	schemaMap := make(map[string]customfield.FieldSchema, len(systemFieldSchemas))
	for _, sf := range systemFieldSchemas {
		schemaMap[sf.Slug] = sf
	}

	for key, value := range pluginFields {
		schema, found := schemaMap[key]
		if !found {
			return fmt.Errorf("%w: %s", ErrUnknownSystemFieldKey, key)
		}
		if err := validateFieldValue(schema, value); err != nil {
			return fmt.Errorf("%w: %s: %v", ErrSystemFieldValidation, schema.Name, err)
		}
	}
	return nil
}

func (s *Service) executeBeforeSaveHook(
	ctx context.Context,
	hookInput []byte,
) ([]byte, error) {
	if s.hookExecutor == nil {
		return nil, nil
	}
	return s.hookExecutor.Execute(ctx, plugin.HookBeforeSave, hookInput)
}

func (s *Service) executeAfterCreateHook(
	ctx context.Context,
	content *Content,
) {
	if s.hookExecutor == nil {
		return
	}
	data := hookData{
		UserID:       content.UserID,
		Title:        content.Title,
		Content:      content.Content,
		Tags:         content.Tags,
		Status:       string(content.Status),
		PostType:     content.PostType,
		CustomFields: content.CustomFields,
	}
	b, _ := json.Marshal(data)
	_, _ = s.hookExecutor.Execute(ctx, plugin.HookAfterCreate, b)
}

func (s *Service) executeAfterPublishHook(
	ctx context.Context,
	content *Content,
) {
	if s.hookExecutor == nil {
		return
	}
	data := hookData{
		UserID:       content.UserID,
		Title:        content.Title,
		Content:      content.Content,
		Tags:         content.Tags,
		Status:       string(content.Status),
		PostType:     content.PostType,
		CustomFields: content.CustomFields,
	}
	b, _ := json.Marshal(data)
	_, _ = s.hookExecutor.Execute(ctx, plugin.HookAfterPublish, b)
}

func (s *Service) SetSystemFields(
	ctx context.Context,
	contentID int,
	systemFields map[string]any,
) (*Content, error) {
	if s.postTypeService == nil {
		return nil, fmt.Errorf("system fields not supported: post type service is nil")
	}

	existing, err := s.repo.GetByID(ctx, contentID)
	if err != nil {
		if errors.Is(err, ErrContentNotFound) {
			return nil, fmt.Errorf("%w: %w", ErrContentNotFound, err)
		}
		return nil, fmt.Errorf("failed to get content: %w", err)
	}

	systemFieldSchemas, err := s.postTypeService.GetSystemFieldsByPostType(existing.PostType)
	if err != nil {
		return nil, fmt.Errorf("failed to get system field schemas: %w", err)
	}

	for key := range systemFields {
		found := false
		for _, sf := range systemFieldSchemas {
			if sf.Slug == key {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("%w: %s", ErrUnknownSystemFieldKey, key)
		}
	}

	for _, sf := range systemFieldSchemas {
		value, exists := systemFields[sf.Slug]
		if !exists {
			continue
		}
		if err := validateFieldValue(sf, value); err != nil {
			return nil, fmt.Errorf("%w: %s: %v", ErrSystemFieldValidation, sf.Name, err)
		}
	}

	if existing.CustomFields == nil {
		existing.CustomFields = make(map[string]any)
	}
	maps.Copy(existing.CustomFields, systemFields)

	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, fmt.Errorf("failed to update content: %w", err)
	}

	return existing, nil
}

func (s *Service) Create(ctx context.Context, userID int, req CreateContentRequest) (*Content, error) {
	if err := ValidateTitle(req.Title); err != nil {
		return nil, fmt.Errorf("title validation failed: %w", err)
	}

	if err := ValidateContent(req.Content); err != nil {
		return nil, fmt.Errorf("content validation failed: %w", err)
	}

	if !req.Status.IsValid() {
		return nil, fmt.Errorf("%w: %s", ErrInvalidStatus, req.Status)
	}

	var err error
	var tags []string
	tags, err = ValidateTags(req.Tags)
	if err != nil {
		return nil, fmt.Errorf("tags validation failed: %w", err)
	}

	slug := s.GenerateSlug(req.Title)

	defaultLanguage := "en"
	language := req.Language
	if language == "" {
		language = defaultLanguage
	}

	// Validate translation group if provided
	if req.TranslationGroupID != nil {
		exists, err := s.repo.TranslationGroupExists(ctx, *req.TranslationGroupID)
		if err != nil {
			return nil, fmt.Errorf("failed to check translation group: %w", err)
		}
		if !exists {
			return nil, ErrTranslationGroupNotFound
		}
	}

	var unique bool
	unique, err = s.repo.CheckSlugUnique(ctx, slug, language)
	if err != nil {
		return nil, fmt.Errorf("failed to check slug uniqueness: %w", err)
	}
	if !unique {
		return nil, fmt.Errorf("%w: %s", ErrSlugAlreadyExists, slug)
	}

	// Default post type to 'post' if not provided
	postType := req.PostType
	if postType == "" {
		postType = "post"
	}

	// Validate post type if postTypeService is available
	if s.postTypeService != nil {
		_, err = s.postTypeService.GetBySlug(postType)
		if err != nil {
			return nil, fmt.Errorf("post type validation failed: %w", err)
		}
	}

	// Capture pre-hook system field values
	preHookSystemValues := s.getSystemFieldValues(postType, req.CustomFields)

	// Execute BeforeSave hook with full data including any system fields
	hookInput := s.buildHookData(userID, req)
	hookResult, hookErr := s.executeBeforeSaveHook(ctx, hookInput)
	if hookErr != nil {
		return nil, fmt.Errorf("before_save hook failed: %w", hookErr)
	}
	if hookResult != nil {
		s.applyHookResult(&req, hookResult)
	}

	// Detect which system fields the plugin added/modified
	pluginSystemFields := s.diffSystemFieldValues(preHookSystemValues, s.getSystemFieldValues(postType, req.CustomFields))

	// Validate plugin-set system field values
	if err := s.validatePluginSystemFields(postType, pluginSystemFields); err != nil {
		return nil, fmt.Errorf("plugin system field validation failed: %w", err)
	}

	s.stripSystemFields(postType, req.CustomFields)

	// Merge plugin-set system field values back after stripping
	if len(pluginSystemFields) > 0 {
		if req.CustomFields == nil {
			req.CustomFields = make(map[string]any)
		}
		maps.Copy(req.CustomFields, pluginSystemFields)
	}

	if err := s.validateCustomFields(postType, req.CustomFields); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCustomFieldValidation, err)
	}

	allowComments := true
	if req.AllowComments != nil {
		allowComments = *req.AllowComments
	} else if postType == "page" {
		allowComments = false
	}

	content := &Content{
		UserID:             userID,
		Title:              strings.TrimSpace(req.Title),
		Slug:               slug,
		Content:            req.Content,
		Tags:               tags,
		Status:             req.Status,
		PostType:           postType,
		MetaDescription:    req.MetaDescription,
		OGTitle:            req.OGTitle,
		OGDescription:      req.OGDescription,
		AllowComments:      allowComments,
		CustomFields:       req.CustomFields,
		Language:           language,
		TranslationGroupID: req.TranslationGroupID,
	}

	// Creating directly as published runs the publish pipeline: auto-generate
	// SEO metadata (honoring any overrides, like Update) so it lands in the
	// initial insert, then fire AfterPublish after the row is persisted. This
	// makes create-when-published behave like create + publish.
	if req.Status == StatusPublished && s.seoService != nil {
		generated, err := s.generateSEOMetadata(content, req.MetaDescription, req.OGTitle, req.OGDescription)
		if err != nil {
			return nil, fmt.Errorf("failed to generate SEO metadata: %w", err)
		}
		content.MetaDescription = generated.MetaDescription
		content.OGTitle = generated.OGTitle
		content.OGDescription = generated.OGDescription
	}

	if err := s.repo.Create(ctx, content); err != nil {
		return nil, fmt.Errorf("failed to create content: %w", err)
	}

	// Execute AfterCreate hook (fire and forget)
	s.executeAfterCreateHook(ctx, content)

	// Fire AfterPublish too when created directly as published, so plugins see
	// the same draft→publish edge they get from the publish endpoint.
	if req.Status == StatusPublished {
		s.executeAfterPublishHook(ctx, content)
	}

	return content, nil
}

func (s *Service) GenerateSlug(title string) string {
	slug := strings.TrimSpace(title)
	slug = strings.ToLower(slug)
	slug = strings.ReplaceAll(slug, " ", "-")

	var result strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	slug = result.String()

	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}

	slug = strings.Trim(slug, "-")

	if len(slug) > 200 {
		slug = slug[:200]
	}

	if slug == "" {
		slug = "untitled"
	}

	return slug
}

func (s *Service) GetBySlug(ctx context.Context, slug string) (*Content, error) {
	if err := ValidateSlug(slug); err != nil {
		return nil, fmt.Errorf("slug validation failed: %w", err)
	}

	content, err := s.repo.GetBySlug(ctx, slug, "en")
	if err != nil {
		return nil, fmt.Errorf("failed to get content: %w", err)
	}

	return content, nil
}

func (s *Service) GetByID(ctx context.Context, id int) (*Content, error) {
	content, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get content: %w", err)
	}

	return content, nil
}

func (s *Service) GetByUser(ctx context.Context, userID int, limit int, offset int) ([]*Content, error) {
	contents, err := s.repo.GetByUser(ctx, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get user content: %w", err)
	}

	return contents, nil
}

func (s *Service) GetAll(ctx context.Context, limit int, offset int) ([]*Content, error) {
	contents, err := s.repo.GetAll(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get all content: %w", err)
	}

	return contents, nil
}

func (s *Service) ListByCursor(ctx context.Context, userID int, limit int, beforeID int, filters ContentFilters) ([]*Content, error) {
	contents, err := s.repo.ListByCursor(ctx, userID, limit, beforeID, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to list content: %w", err)
	}

	return contents, nil
}

func (s *Service) ListByFilters(ctx context.Context, userID int, filters ContentFilters) ([]*Content, error) {
	if filters.Limit <= 0 {
		filters.Limit = 100
	}
	if filters.Limit > 1000 {
		filters.Limit = 1000
	}
	if filters.Offset < 0 {
		filters.Offset = 0
	}

	for _, f := range filters.CustomFieldFilters {
		if err := ValidateCustomFieldFilter(f); err != nil {
			return nil, fmt.Errorf("invalid custom field filter: %w", err)
		}
	}

	contents, err := s.repo.ListByFilters(ctx, userID, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to list content by filters: %w", err)
	}

	return contents, nil
}

func (s *Service) GenerateSlugFromTitle(ctx context.Context, title string) (string, error) {
	if err := ValidateTitle(title); err != nil {
		return "", fmt.Errorf("title validation failed: %w", err)
	}

	slug := s.GenerateSlug(title)
	return slug, nil
}

func (s *Service) GetPublished(ctx context.Context, limit int, offset int) ([]*Content, error) {
	contents, err := s.repo.GetPublished(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get published content: %w", err)
	}
	return contents, nil
}

func (s *Service) GetTranslations(ctx context.Context, translationGroupID int, excludeID int) ([]*Content, error) {
	return s.repo.GetTranslations(ctx, translationGroupID, excludeID)
}

func (s *Service) GetPublishedBySlug(ctx context.Context, slug string, language string) (*Content, error) {
	if language == "" {
		language = "en"
	}

	if err := ValidateSlug(slug); err != nil {
		return nil, fmt.Errorf("slug validation failed: %w", err)
	}

	content, err := s.repo.GetPublishedBySlug(ctx, slug, language)
	if err != nil {
		return nil, fmt.Errorf("failed to get published content: %w", err)
	}
	return content, nil
}

func (s *Service) GetPublishedBySlugAny(ctx context.Context, slug string) (*Content, error) {
	if err := ValidateSlug(slug); err != nil {
		return nil, fmt.Errorf("slug validation failed: %w", err)
	}

	content, err := s.repo.GetPublishedBySlugAny(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("failed to get published content: %w", err)
	}
	return content, nil
}

func (s *Service) GetPublishedByID(ctx context.Context, id int) (*Content, error) {
	content, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get published content by id: %w", err)
	}
	if content.Status != StatusPublished {
		return nil, ErrContentNotFound
	}
	return content, nil
}

func (s *Service) GetPublishedByAuthorUsername(ctx context.Context, username string, limit int, offset int) ([]*Content, error) {
	contents, err := s.repo.GetPublishedByAuthorUsername(ctx, username, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get published content by author: %w", err)
	}
	return contents, nil
}

func (s *Service) AuthorExists(ctx context.Context, username string) (bool, error) {
	return s.repo.AuthorExists(ctx, username)
}

func (s *Service) GetPublishedByTag(ctx context.Context, tag string, limit int, offset int) ([]*Content, error) {
	contents, err := s.repo.GetPublishedByTag(ctx, tag, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get published content by tag: %w", err)
	}
	return contents, nil
}

func (s *Service) GetPublishedPages(ctx context.Context) ([]*Content, error) {
	pages, err := s.repo.GetPublishedPages(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get published pages: %w", err)
	}
	return pages, nil
}

func (s *Service) GetPublishedCustomPostTypes(ctx context.Context) ([]string, error) {
	postTypes, err := s.repo.GetPublishedCustomPostTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get published custom post types: %w", err)
	}
	return postTypes, nil
}

func (s *Service) GetPublishedByPostType(ctx context.Context, postType string, limit int, offset int) ([]*Content, error) {
	contents, err := s.repo.GetPublishedByPostType(ctx, postType, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get published content by post type: %w", err)
	}
	return contents, nil
}

func (s *Service) SearchPublished(ctx context.Context, query string, limit int) ([]*Content, error) {
	query = strings.TrimSpace(query)
	if len(query) < 2 {
		return []*Content{}, nil
	}
	contents, err := s.repo.SearchPublished(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search published content: %w", err)
	}
	return contents, nil
}

const (
	// defaultRelatedLimit is the number of related posts returned when no limit is requested.
	defaultRelatedLimit = 5
	// maxRelatedLimit caps the number of related posts to keep queries bounded.
	maxRelatedLimit = 20
)

// GetRelated returns up to limit related posts for the content identified by id.
// Related posts share at least one tag, the same post type, and the same language
// as the source, ranked by the number of shared tags then by recency. When there
// are not enough tag-overlap matches (or the source has no tags), the result is
// backfilled with the latest published posts of the same post type and language.
// The source post is always excluded.
func (s *Service) GetRelated(ctx context.Context, id int, limit int) ([]*Content, error) {
	if limit <= 0 {
		limit = defaultRelatedLimit
	}
	if limit > maxRelatedLimit {
		limit = maxRelatedLimit
	}

	source, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get content: %w", err)
	}

	related, err := s.repo.GetRelatedByTags(ctx, id, source.Tags, source.PostType, source.Language, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get related content by tags: %w", err)
	}

	if len(related) >= limit {
		return related, nil
	}

	latest, err := s.repo.GetLatestByPostType(ctx, id, source.PostType, source.Language, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest content by post type: %w", err)
	}

	seen := make(map[int]bool, len(related)+1)
	seen[id] = true
	for _, c := range related {
		seen[c.ID] = true
	}

	for _, c := range latest {
		if len(related) >= limit {
			break
		}
		if seen[c.ID] {
			continue
		}
		seen[c.ID] = true
		related = append(related, c)
	}

	return related, nil
}

func (s *Service) Update(ctx context.Context, id int, userID int, role string, req UpdateContentRequest) (*Content, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get content: %w", err)
	}

	if existing.UserID != userID && role != RoleAdmin {
		return nil, ErrUnauthorized
	}

	if err := ValidateTitle(req.Title); err != nil {
		return nil, fmt.Errorf("title validation failed: %w", err)
	}

	if err := ValidateContent(req.Content); err != nil {
		return nil, fmt.Errorf("content validation failed: %w", err)
	}

	if !req.Status.IsValid() {
		return nil, fmt.Errorf("%w: %s", ErrInvalidStatus, req.Status)
	}

	var tags []string
	tags, err = ValidateTags(req.Tags)
	if err != nil {
		return nil, fmt.Errorf("tags validation failed: %w", err)
	}

	// Validate post type if postTypeService is available
	if s.postTypeService != nil && req.PostType != "" {
		_, err := s.postTypeService.GetBySlug(req.PostType)
		if err != nil {
			return nil, fmt.Errorf("post type validation failed: %w", err)
		}
	}

	effectivePostType := existing.PostType
	if req.PostType != "" {
		effectivePostType = req.PostType
	}
	if effectivePostType == "" {
		effectivePostType = "post"
	}

	// Capture pre-hook system field values from the combined view
	mergedCustomFields := make(map[string]any)
	if req.CustomFields != nil {
		maps.Copy(mergedCustomFields, req.CustomFields)
	}
	if existing.CustomFields != nil {
		maps.Copy(mergedCustomFields, existing.CustomFields)
	}
	preHookSystemValues := s.getSystemFieldValues(effectivePostType, mergedCustomFields)

	// Execute BeforeSave hook with full data including existing system fields
	hookInput := s.buildHookDataForUpdate(existing, req)
	hookResult, hookErr := s.executeBeforeSaveHook(ctx, hookInput)
	if hookErr != nil {
		return nil, fmt.Errorf("before_save hook failed: %w", hookErr)
	}
	if hookResult != nil {
		s.applyHookResultForUpdate(&req, hookResult)
	}

	// Detect which system fields the plugin added/modified.
	// Copy existing first, then req on top so hook changes in req take precedence.
	postHookMerged := make(map[string]any)
	if existing.CustomFields != nil {
		maps.Copy(postHookMerged, existing.CustomFields)
	}
	if req.CustomFields != nil {
		maps.Copy(postHookMerged, req.CustomFields)
	}
	pluginSystemFields := s.diffSystemFieldValues(preHookSystemValues, s.getSystemFieldValues(effectivePostType, postHookMerged))

	// Validate plugin-set system field values
	if err := s.validatePluginSystemFields(effectivePostType, pluginSystemFields); err != nil {
		return nil, fmt.Errorf("plugin system field validation failed: %w", err)
	}

	s.stripSystemFields(effectivePostType, req.CustomFields)

	if err := s.validateCustomFields(effectivePostType, req.CustomFields); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCustomFieldValidation, err)
	}

	newTitle := strings.TrimSpace(req.Title)
	if newTitle != existing.Title {
		newSlug := s.GenerateSlug(newTitle)
		if newSlug != existing.Slug {
			unique, slugErr := s.repo.CheckSlugUnique(ctx, newSlug, existing.Language)
			if slugErr != nil {
				return nil, fmt.Errorf("failed to check slug uniqueness: %w", slugErr)
			}
			if !unique {
				return nil, fmt.Errorf("%w: %s", ErrSlugAlreadyExists, newSlug)
			}
			existing.Slug = newSlug
		}
	}
	existing.Title = newTitle
	existing.Content = req.Content
	existing.Tags = tags
	existing.PostType = req.PostType
	if req.CustomFields != nil {
		// Preserve system field values from existing content
		var systemValues map[string]any
		if existing.CustomFields != nil && s.postTypeService != nil {
			if schemas, err := s.postTypeService.GetSystemFieldsByPostType(effectivePostType); err == nil {
				for _, sf := range schemas {
					if v, ok := existing.CustomFields[sf.Slug]; ok {
						if systemValues == nil {
							systemValues = make(map[string]any)
						}
						systemValues[sf.Slug] = v
					}
				}
			}
		}
		existing.CustomFields = req.CustomFields
		for slug, val := range systemValues {
			if _, isPluginField := pluginSystemFields[slug]; !isPluginField {
				existing.CustomFields[slug] = val
			}
		}
		if len(pluginSystemFields) > 0 {
			maps.Copy(existing.CustomFields, pluginSystemFields)
		}
	}

	if req.AllowComments != nil {
		existing.AllowComments = *req.AllowComments
	}

	// Auto-generate SEO metadata when transitioning to published status
	isTransitioningToPublished := existing.Status != StatusPublished && req.Status == StatusPublished
	existing.Status = req.Status

	if isTransitioningToPublished && s.seoService != nil {
		generated, err := s.generateSEOMetadata(existing, req.MetaDescription, req.OGTitle, req.OGDescription)
		if err != nil {
			return nil, fmt.Errorf("failed to generate SEO metadata: %w", err)
		}
		existing.MetaDescription = generated.MetaDescription
		existing.OGTitle = generated.OGTitle
		existing.OGDescription = generated.OGDescription
	} else {
		// Use provided request values (may be empty strings)
		existing.MetaDescription = req.MetaDescription
		existing.OGTitle = req.OGTitle
		existing.OGDescription = req.OGDescription
	}

	existing.UpdatedBy = userID

	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, fmt.Errorf("failed to update content: %w", err)
	}

	// Execute AfterPublish hook on status transition (fire and forget)
	if isTransitioningToPublished {
		s.executeAfterPublishHook(ctx, existing)
	}

	return existing, nil
}

// generateSEOMetadata builds the SEO input from content (title/content/slug-URL/
// timestamps/author/tags), applies any non-empty overrides, and generates the
// metadata. Pass empty strings for the overrides to get pure auto-generation
// (the status-only publish path). Shared by Create, Update, and transitionStatus
// so every publish edge produces identical SEO.
func (s *Service) generateSEOMetadata(
	c *Content,
	metaDescription, ogTitle, ogDescription string,
) (*seo.GeneratedMetadata, error) {
	now := time.Now().Format(time.RFC3339)
	urlPrefix := "/posts"
	if c.PostType != "" && c.PostType != "post" {
		urlPrefix = fmt.Sprintf("/%s", c.PostType)
	}
	seoInput := seo.GenerateInput{
		Title:         c.Title,
		Content:       c.Content,
		URL:           fmt.Sprintf("%s/%s", urlPrefix, c.Slug),
		DatePublished: now,
		DateModified:  now,
		AuthorName:    c.Author,
		Tags:          c.Tags,
	}
	if metaDescription != "" {
		seoInput.MetaDescription = metaDescription
	}
	if ogTitle != "" {
		seoInput.OGTitle = ogTitle
	}
	if ogDescription != "" {
		seoInput.OGDescription = ogDescription
	}
	return s.seoService.Generate(seoInput)
}

// transitionStatus flips existing.Status to newStatus, persists the row, and
// on the draft→published edge (and only on that edge) runs SEO auto-generation
// (if seoService is configured) and fires the AfterPublish plugin hook. SEO
// overrides are intentionally NOT honored here — the publish/unpublish path is
// status-only, so the auto-generated SEO is the single source of truth (the
// Update path takes overrides from the request; that branch is inlined above
// and not refactored, to keep this helper minimal and the Update tests stable).
//
// The caller is responsible for pre-fetching the content and enforcing
// ownership/role checks (mirroring Update's pattern). On persistence failure
// the returned error wraps the underlying repo error; on SEO failure the
// generated error is returned WITHOUT firing the hook, so a failed SEO gen
// does not silently publish.
func (s *Service) transitionStatus(
	ctx context.Context,
	existing *Content,
	newStatus Status,
	userID int,
) error {
	isTransitioningToPublished := existing.Status != StatusPublished && newStatus == StatusPublished
	previousStatus := existing.Status
	existing.Status = newStatus

	if isTransitioningToPublished && s.seoService != nil {
		generated, err := s.generateSEOMetadata(existing, "", "", "")
		if err != nil {
			return fmt.Errorf("failed to generate SEO metadata: %w", err)
		}
		existing.MetaDescription = generated.MetaDescription
		existing.OGTitle = generated.OGTitle
		existing.OGDescription = generated.OGDescription
	}

	existing.UpdatedBy = userID

	if err := s.repo.Update(ctx, existing); err != nil {
		// Roll the in-memory status back so the caller's view stays consistent
		// with the not-yet-persisted row (useful for tests + for callers that
		// re-read existing.Status after an error).
		existing.Status = previousStatus
		return fmt.Errorf("failed to update content: %w", err)
	}

	if isTransitioningToPublished {
		s.executeAfterPublishHook(ctx, existing)
	}

	return nil
}

// Publish flips a content item's status to StatusPublished via transitionStatus.
// Ownership/role checks mirror Update: a non-admin caller must be the owner or
// the service returns ErrUnauthorized. An already-published post is a no-op
// (200 idempotent: the row is persisted unchanged, no hook fires, no SEO regen
// runs) — only the actual draft→published edge triggers SEO + hook work.
//
// Errors:
//   - ErrContentNotFound when the id does not exist
//   - ErrUnauthorized when the caller is neither owner nor admin
//   - wrapped repo / SEO errors on persistence / generation failure
func (s *Service) Publish(ctx context.Context, id int, userID int, role string) (*Content, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get content: %w", err)
	}

	if existing.UserID != userID && role != RoleAdmin {
		return nil, ErrUnauthorized
	}

	if err := s.transitionStatus(ctx, existing, StatusPublished, userID); err != nil {
		return nil, err
	}

	return existing, nil
}

// Unpublish flips a content item's status to StatusDraft via transitionStatus.
// Same ownership/role contract as Publish. An already-draft post is a no-op
// (idempotent: row is persisted unchanged, no hook fires, no SEO regen runs).
//
// Errors:
//   - ErrContentNotFound when the id does not exist
//   - ErrUnauthorized when the caller is neither owner nor admin
//   - wrapped repo errors on persistence failure
func (s *Service) Unpublish(ctx context.Context, id int, userID int, role string) (*Content, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get content: %w", err)
	}

	if existing.UserID != userID && role != RoleAdmin {
		return nil, ErrUnauthorized
	}

	if err := s.transitionStatus(ctx, existing, StatusDraft, userID); err != nil {
		return nil, err
	}

	return existing, nil
}

func (s *Service) SubmitComment(ctx context.Context, contentID int, userID int, req CreateCommentRequest) (*Comment, error) {
	if err := ValidateCommentText(req.Comment); err != nil {
		return nil, fmt.Errorf("comment validation failed: %w", err)
	}

	content, err := s.repo.GetByID(ctx, contentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get content: %w", err)
	}

	if !content.AllowComments {
		return nil, ErrUnauthorized
	}

	comment := &Comment{
		ContentID: contentID,
		UserID:    userID,
		Comment:   strings.TrimSpace(req.Comment),
		Status:    CommentStatusPending,
	}

	if err := s.commentRepo.Create(ctx, comment); err != nil {
		return nil, fmt.Errorf("failed to create comment: %w", err)
	}

	return comment, nil
}

func (s *Service) GetCommentsByUserID(ctx context.Context, userID int) ([]*Comment, error) {
	comments, err := s.commentRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get comments by user: %w", err)
	}

	return comments, nil
}

func (s *Service) GetCommentsForContent(ctx context.Context, contentID int) ([]*Comment, error) {
	comments, err := s.commentRepo.GetByContentID(ctx, contentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get comments: %w", err)
	}

	return comments, nil
}

func (s *Service) GetCommentsForModeration(ctx context.Context, contentID int) ([]*Comment, error) {
	comments, err := s.commentRepo.GetByContentIDForModeration(ctx, contentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get comments for moderation: %w", err)
	}

	return comments, nil
}

func (s *Service) GetCommentsByStatus(ctx context.Context, status CommentStatus) ([]*Comment, error) {
	comments, err := s.commentRepo.GetByStatus(ctx, status)
	if err != nil {
		return nil, fmt.Errorf("failed to get comments by status: %w", err)
	}

	return comments, nil
}

func (s *Service) ApproveComment(ctx context.Context, commentID int) error {
	if err := s.commentRepo.UpdateStatus(ctx, commentID, CommentStatusApproved); err != nil {
		return fmt.Errorf("failed to approve comment: %w", err)
	}

	return nil
}

func (s *Service) RejectComment(ctx context.Context, commentID int) error {
	if err := s.commentRepo.UpdateStatus(ctx, commentID, CommentStatusRejected); err != nil {
		return fmt.Errorf("failed to reject comment: %w", err)
	}

	return nil
}

func (s *Service) MarkAsSpam(ctx context.Context, commentID int) error {
	if err := s.commentRepo.UpdateStatus(ctx, commentID, CommentStatusSpam); err != nil {
		return fmt.Errorf("failed to mark comment as spam: %w", err)
	}

	return nil
}

func (s *Service) DeleteContent(ctx context.Context, id int, userID int, role string) error {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get content: %w", err)
	}

	if existing.UserID != userID && role != RoleAdmin {
		return ErrUnauthorized
	}

	if role == RoleAdmin {
		if err := s.repo.DeleteByID(ctx, id); err != nil {
			return fmt.Errorf("failed to delete content: %w", err)
		}
	} else {
		if err := s.repo.Delete(ctx, id, userID); err != nil {
			return fmt.Errorf("failed to delete content: %w", err)
		}
	}

	return nil
}

func (s *Service) DeleteComment(ctx context.Context, commentID int) error {
	if err := s.commentRepo.Delete(ctx, commentID); err != nil {
		return fmt.Errorf("failed to delete comment: %w", err)
	}

	return nil
}

func (s *Service) DeleteOwnComment(ctx context.Context, commentID int, userID int) error {
	comment, err := s.commentRepo.GetByID(ctx, commentID)
	if err != nil {
		return ErrCommentNotFound
	}

	if comment.UserID != userID {
		return ErrCommentNotFound
	}

	if err := s.commentRepo.Delete(ctx, commentID); err != nil {
		return fmt.Errorf("failed to delete comment: %w", err)
	}

	return nil
}

func (s *Service) DeleteCommentsByUserID(ctx context.Context, userID int) error {
	if err := s.commentRepo.DeleteByUserID(ctx, userID); err != nil {
		return fmt.Errorf("failed to delete comments by user: %w", err)
	}

	return nil
}

func (s *Service) GetComment(ctx context.Context, commentID int) (*Comment, error) {
	comment, err := s.commentRepo.GetByID(ctx, commentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get comment: %w", err)
	}

	return comment, nil
}

func (s *Service) UpdateCommentStatus(ctx context.Context, commentID int, status CommentStatus) error {
	if err := ValidateCommentStatus(status); err != nil {
		return fmt.Errorf("invalid comment status: %w", err)
	}

	if err := s.commentRepo.UpdateStatus(ctx, commentID, status); err != nil {
		return fmt.Errorf("failed to update comment status: %w", err)
	}

	return nil
}

func NewService(
	repo Repository,
	seoService *seo.Service,
	postTypeService PostTypeServiceInterface,
) *Service {
	return &Service{
		repo:            repo,
		seoService:      seoService,
		postTypeService: postTypeService,
	}
}

func NewServiceWithComments(
	repo Repository,
	commentRepo CommentRepository,
	seoService *seo.Service,
	postTypeService PostTypeServiceInterface,
) *Service {
	return &Service{
		repo:            repo,
		commentRepo:     commentRepo,
		seoService:      seoService,
		postTypeService: postTypeService,
	}
}

func NewServiceWithHooks(
	repo Repository,
	commentRepo CommentRepository,
	seoService *seo.Service,
	postTypeService PostTypeServiceInterface,
	hookExecutor HookExecutor,
) *Service {
	return &Service{
		repo:            repo,
		commentRepo:     commentRepo,
		seoService:      seoService,
		postTypeService: postTypeService,
		hookExecutor:    hookExecutor,
	}
}
