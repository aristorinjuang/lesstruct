package capability

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// LoadManifest reads a TOML capability manifest from the given path.
// Returns nil, nil if the file does not exist (missing manifest = no capabilities).
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading manifest %q: %w", path, err)
	}

	var m Manifest
	if err := toml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest %q: %w", path, err)
	}

	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("validating manifest %q: %w", path, err)
	}

	return &m, nil
}