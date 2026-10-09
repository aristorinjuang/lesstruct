package thumbnail

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
)

// Service caches thumbnail configurations and provides thread-safe access.
type Service struct {
	configs []ThumbnailConfig
	mu      sync.RWMutex
}

// GetAll returns all cached thumbnail configs.
func (s *Service) GetAll() []ThumbnailConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]ThumbnailConfig, len(s.configs))
	copy(result, s.configs)
	return result
}

// GetBySuffix returns the thumbnail config matching the given suffix.
func (s *Service) GetBySuffix(suffix string) (ThumbnailConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, c := range s.configs {
		if c.Suffix == suffix {
			return c, nil
		}
	}
	return ThumbnailConfig{}, ErrSuffixNotFound
}

// LoadConfigFromFile reads a TOML file and loads thumbnail configs into the service.
// If the file does not exist, defaults (already cached) are kept and no error is returned.
func (s *Service) LoadConfigFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading thumbnail config %s: %w", path, err)
	}

	var cfg struct {
		Thumbnails []ThumbnailConfig `toml:"thumbnail"`
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parsing thumbnail config %s: %w", path, err)
	}

	for _, tc := range cfg.Thumbnails {
		if err := tc.Validate(); err != nil {
			return fmt.Errorf("validating thumbnail config %s: %w", path, err)
		}
	}

	if err := ValidateUnique(cfg.Thumbnails); err != nil {
		return fmt.Errorf("validating thumbnail config %s: %w", path, err)
	}

	if len(cfg.Thumbnails) == 0 {
		return nil
	}

	for i := range cfg.Thumbnails {
		cfg.Thumbnails[i].Suffix = strings.TrimSpace(cfg.Thumbnails[i].Suffix)
	}

	s.mu.Lock()
	s.configs = cfg.Thumbnails
	s.mu.Unlock()

	return nil
}

// NewService creates a Service pre-loaded with default thumbnail config.
func NewService() *Service {
	return &Service{
		configs: []ThumbnailConfig{
			{
				MaxWidth: 370,
				Suffix:   "_thumb",
			},
		},
	}
}
