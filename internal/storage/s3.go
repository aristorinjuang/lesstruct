package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// contentTypeFor returns the MIME type for a file, derived from the extension
// with a content-sniffing fallback
func contentTypeFor(filename string, data []byte) string {
	if contentType := mime.TypeByExtension(filepath.Ext(filename)); contentType != "" {
		return contentType
	}
	return http.DetectContentType(data)
}

// S3Options holds the configuration for an S3-compatible storage backend.
// AWS S3 and MinIO both speak the S3 API — the same client serves either:
// leave Endpoint empty for AWS, or set it (e.g. "http://minio:9000") for MinIO.
type S3Options struct {
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	UsePathStyle    bool
	PublicBaseURL   string
	KeyPrefix       string
	HTTPClient      *http.Client
	Retryer         aws.Retryer
}

// S3Storage implements the Storage interface for S3-compatible object storage
type S3Storage struct {
	client        *s3.Client
	bucket        string
	keyPrefix     string
	publicBaseURL string
}

// keyFor resolves a stored path or bare filename to a full object key
func (s *S3Storage) keyFor(filePath string) string {
	if strings.HasPrefix(filePath, s.keyPrefix) {
		return filePath
	}
	return s.keyPrefix + filePath
}

// Save uploads a file to the bucket under the key prefix + filename
func (s *S3Storage) Save(filename string, reader io.Reader) (string, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read upload data: %w", err)
	}

	key := s.keyPrefix + filename
	_, err = s.client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentTypeFor(filename, data)),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload object %q: %w", key, err)
	}

	return key, nil
}

// Delete removes an object from the bucket
func (s *S3Storage) Delete(filePath string) error {
	key := s.keyFor(filePath)
	_, err := s.client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete object %q: %w", key, err)
	}
	return nil
}

// GetURL returns the public URL for a stored file
func (s *S3Storage) GetURL(filePath string) string {
	return strings.TrimRight(s.publicBaseURL, "/") + "/" + s.keyFor(filePath)
}

// NewS3Storage creates an S3-compatible storage backend (AWS S3 or MinIO)
func NewS3Storage(opts S3Options) (*S3Storage, error) {
	if opts.Bucket == "" {
		return nil, fmt.Errorf("s3 storage: bucket is required")
	}
	if opts.Region == "" {
		return nil, fmt.Errorf("s3 storage: region is required")
	}
	if opts.PublicBaseURL == "" {
		return nil, fmt.Errorf("s3 storage: public base URL is required")
	}

	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{}
	}

	awsConfig := aws.Config{
		Region:      opts.Region,
		Credentials: credentials.NewStaticCredentialsProvider(opts.AccessKeyID, opts.SecretAccessKey, ""),
		HTTPClient:  httpClient,
	}

	client := s3.NewFromConfig(awsConfig, func(o *s3.Options) {
		o.UsePathStyle = opts.UsePathStyle
		if opts.Retryer != nil {
			o.Retryer = opts.Retryer
		}
		if opts.Endpoint != "" {
			o.BaseEndpoint = aws.String(opts.Endpoint)
		}
	})

	return &S3Storage{
		client:        client,
		bucket:        opts.Bucket,
		keyPrefix:     opts.KeyPrefix,
		publicBaseURL: opts.PublicBaseURL,
	}, nil
}
