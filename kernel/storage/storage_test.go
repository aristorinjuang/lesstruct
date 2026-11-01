package storage_test

import (
	"bytes"
	"io"
	"testing"

	kernelstorage "github.com/aristorinjuang/lesstruct/kernel/storage"
	"github.com/stretchr/testify/assert"
)

type stubStorage struct {
	saved map[string][]byte
}

func (s *stubStorage) Save(filename string, reader io.Reader) (string, error) {
	if s.saved == nil {
		s.saved = make(map[string][]byte)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	s.saved[filename] = data
	return filename, nil
}

func (s *stubStorage) Delete(filePath string) error {
	delete(s.saved, filePath)
	return nil
}

func (s *stubStorage) GetURL(filePath string) string {
	return "/uploads/" + filePath
}

func (s *stubStorage) Open(filePath string) (io.ReadCloser, error) {
	data, ok := s.saved[filePath]
	if !ok {
		return nil, io.ErrUnexpectedEOF
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func TestStorageContract(t *testing.T) {
	var storage kernelstorage.Storage = &stubStorage{}

	assert.NotNil(t, storage)
}
