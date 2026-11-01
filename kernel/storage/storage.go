// Package storage defines the file-storage contract for media-like resources.
// Implementations (local disk, S3/MinIO) live in core until their extraction
// as infrastructure-provider modules; they satisfy this interface
// structurally, so no adapter is needed on either side.
package storage

import (
	"io"
)

// Storage persists uploaded bytes under opaque keys. Save returns the storage
// key (local path or object key); Delete removes it; GetURL renders the
// public URL for a stored key. Open returns a reader for the stored bytes so
// maintenance jobs (one-time library re-encode) read back through the
// interface instead of sniffing the driver. Implementations must be safe for
// concurrent use; all methods fail closed with descriptive errors.
type Storage interface {
	Save(filename string, reader io.Reader) (string, error)
	Delete(filePath string) error
	GetURL(filePath string) string
	Open(filePath string) (io.ReadCloser, error)
}
