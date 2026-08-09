package storage

import "io"

// Storage defines the interface for file storage operations
type Storage interface {
	Save(filename string, reader io.Reader) (string, error)
	Delete(filePath string) error
	GetURL(filePath string) string
}
