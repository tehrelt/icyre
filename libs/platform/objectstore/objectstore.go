// Package objectstore is the S3-compatible object storage adapter (MinIO
// locally, any S3 in production): presigned upload/download, stat with
// checksums, and bucket bootstrap. It knows nothing about tracks or covers —
// key layout lives in libs/contracts/media.
package objectstore

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// ErrNotFound means the object does not exist.
var ErrNotFound = errors.New("object not found")

// Config configures a Store.
type Config struct {
	// Endpoint is host:port as seen by the service (e.g. minio:9000).
	Endpoint string
	Secure   bool
	// PublicEndpoint is host:port as seen by clients (CDN/origin); signed
	// URLs are issued for it. Defaults to Endpoint.
	PublicEndpoint string
	PublicSecure   bool
	AccessKey      string
	SecretKey      string
	// Region is fixed so presigning never needs a network round trip.
	Region string
	// Timeout bounds every storage call (not the transfers clients make
	// with signed URLs).
	Timeout time.Duration
}

// Store talks to one S3-compatible endpoint.
type Store struct {
	cl      *minio.Client
	signer  *minio.Client
	timeout time.Duration
}

// New returns a Store. It does not touch the network.
func New(cfg Config) (*Store, error) {
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	if cfg.PublicEndpoint == "" {
		cfg.PublicEndpoint, cfg.PublicSecure = cfg.Endpoint, cfg.Secure
	}
	creds := credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, "")
	cl, err := minio.New(cfg.Endpoint, &minio.Options{Creds: creds, Secure: cfg.Secure, Region: cfg.Region, TrailingHeaders: true})
	if err != nil {
		return nil, fmt.Errorf("objectstore client: %w", err)
	}
	signer, err := minio.New(cfg.PublicEndpoint, &minio.Options{Creds: creds, Secure: cfg.PublicSecure, Region: cfg.Region})
	if err != nil {
		return nil, fmt.Errorf("objectstore signer: %w", err)
	}
	return &Store{cl: cl, signer: signer, timeout: cfg.Timeout}, nil
}

func (s *Store) ctx(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, s.timeout)
}

// Check is a readiness check: the bucket must exist and be reachable.
func (s *Store) Check(bucket string) func(context.Context) error {
	return func(ctx context.Context) error {
		ctx, cancel := s.ctx(ctx)
		defer cancel()
		ok, err := s.cl.BucketExists(ctx, bucket)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("bucket %s does not exist", bucket)
		}
		return nil
	}
}

// EnsureBucket creates the bucket if it is missing (bootstrap, tests).
func (s *Store) EnsureBucket(ctx context.Context, bucket string) error {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	ok, err := s.cl.BucketExists(ctx, bucket)
	if err != nil || ok {
		return err
	}
	return s.cl.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
}

// ObjectInfo describes a stored object.
type ObjectInfo struct {
	Key          string
	Size         int64
	ContentType  string
	ETag         string
	LastModified time.Time
	// SHA256 is the base64 checksum when the object was stored with one.
	SHA256 string
}

// Stat returns object metadata, or ErrNotFound.
func (s *Store) Stat(ctx context.Context, bucket, key string) (ObjectInfo, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	info, err := s.cl.StatObject(ctx, bucket, key, minio.StatObjectOptions{Checksum: true})
	if err != nil {
		if minio.ToErrorResponse(err).StatusCode == http.StatusNotFound {
			return ObjectInfo{}, ErrNotFound
		}
		return ObjectInfo{}, fmt.Errorf("stat %s: %w", key, err)
	}
	return ObjectInfo{Key: info.Key, Size: info.Size, ContentType: info.ContentType, ETag: info.ETag, LastModified: info.LastModified, SHA256: info.ChecksumSHA256}, nil
}

// PutOptions describe an uploaded object.
type PutOptions struct {
	ContentType  string
	CacheControl string
}

// Put stores an object from a reader, with a SHA-256 checksum the store
// verifies on arrival. For service-side writes (workers, seeding); clients
// upload with PresignUpload.
func (s *Store) Put(ctx context.Context, bucket, key string, r io.Reader, size int64, o PutOptions) (ObjectInfo, error) {
	info, err := s.cl.PutObject(ctx, bucket, key, r, size, minio.PutObjectOptions{
		ContentType: o.ContentType, CacheControl: o.CacheControl,
		Checksum: minio.ChecksumSHA256,
	})
	if err != nil {
		return ObjectInfo{}, fmt.Errorf("put %s: %w", key, err)
	}
	return ObjectInfo{Key: key, Size: info.Size, ETag: info.ETag, SHA256: info.ChecksumSHA256, ContentType: o.ContentType}, nil
}

// SignedURL is a time-limited URL for a direct client transfer.
type SignedURL struct {
	Method string
	URL    string
	// Headers must be sent exactly as given (they are part of the signature).
	Headers   map[string]string
	ExpiresAt time.Time
}

// DownloadOptions tune a presigned GET.
type DownloadOptions struct {
	// CacheControl overrides the response header (e.g. for a CDN).
	CacheControl string
}

// PresignDownload signs a GET for the object. The URL carries no user data
// and supports HTTP Range requests.
func (s *Store) PresignDownload(ctx context.Context, bucket, key string, ttl time.Duration, o DownloadOptions) (SignedURL, error) {
	params := url.Values{}
	if o.CacheControl != "" {
		params.Set("response-cache-control", o.CacheControl)
	}
	now := time.Now()
	u, err := s.signer.PresignedGetObject(ctx, bucket, key, ttl, params)
	if err != nil {
		return SignedURL{}, fmt.Errorf("presign get %s: %w", key, err)
	}
	return SignedURL{Method: http.MethodGet, URL: u.String(), ExpiresAt: now.Add(ttl)}, nil
}

// UploadOptions constrain a presigned PUT.
type UploadOptions struct {
	ContentType string
	// SHA256 is the expected digest of the body. When set, the store
	// rejects a body that does not match (x-amz-checksum-sha256).
	SHA256 []byte
}

// PresignUpload signs a PUT. The client must send the returned headers.
func (s *Store) PresignUpload(ctx context.Context, bucket, key string, ttl time.Duration, o UploadOptions) (SignedURL, error) {
	headers := http.Header{}
	if o.ContentType != "" {
		headers.Set("Content-Type", o.ContentType)
	}
	if len(o.SHA256) > 0 {
		if len(o.SHA256) != sha256.Size {
			return SignedURL{}, errors.New("sha256 must be 32 bytes")
		}
		headers.Set("x-amz-checksum-sha256", base64.StdEncoding.EncodeToString(o.SHA256))
	}
	now := time.Now()
	u, err := s.signer.PresignHeader(ctx, http.MethodPut, bucket, key, ttl, nil, headers)
	if err != nil {
		return SignedURL{}, fmt.Errorf("presign put %s: %w", key, err)
	}
	out := SignedURL{Method: http.MethodPut, URL: u.String(), Headers: map[string]string{}, ExpiresAt: now.Add(ttl)}
	for k := range headers {
		out.Headers[k] = headers.Get(k)
	}
	return out, nil
}

// VerifySHA256 confirms a stored object has the expected digest (e.g. when
// an upload is completed). Objects stored without a checksum fail.
func (s *Store) VerifySHA256(ctx context.Context, bucket, key string, want []byte) error {
	info, err := s.Stat(ctx, bucket, key)
	if err != nil {
		return err
	}
	if info.SHA256 == "" {
		return fmt.Errorf("%s has no stored checksum", key)
	}
	if info.SHA256 != base64.StdEncoding.EncodeToString(want) {
		return fmt.Errorf("%s: checksum mismatch", key)
	}
	return nil
}
