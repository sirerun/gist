package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// s3PutAttempts bounds retries of a conditional put that S3 answers with 409
// ConditionalRequestConflict, which it returns while a concurrent conditional
// write to the same key is still in flight.
const s3PutAttempts = 3

// s3API is the subset of *s3.Client the backend calls.
type s3API interface {
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}
type s3DeleteAPI interface {
	DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

// s3Backend keeps blobs as objects named <prefix>/<digest> in one bucket. S3
// makes an object visible only once its PUT completes, so a failed or
// interrupted put leaves nothing behind. Encryption comes from the bucket
// default; no ACL is set.
type s3Backend struct {
	client s3API
	bucket string
	prefix string
}

func isS3Root(root string) bool { return strings.HasPrefix(root, "s3://") }

// parseS3Root splits s3://bucket/optional/prefix into bucket and a prefix with
// no leading or trailing slash.
func parseS3Root(root string) (bucket, prefix string, err error) {
	u, err := url.Parse(root)
	if err != nil {
		return "", "", fmt.Errorf("objects: parse %q: %w", root, err)
	}
	if u.Scheme != "s3" {
		return "", "", fmt.Errorf("objects: %q is not an s3:// root", root)
	}
	if u.Host == "" {
		return "", "", fmt.Errorf("objects: %q has no bucket", root)
	}
	if u.User != nil || u.Port() != "" || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return "", "", fmt.Errorf("objects: %q must be s3://bucket/prefix only", root)
	}
	prefix = strings.Trim(u.Path, "/")
	for _, seg := range strings.Split(prefix, "/") {
		if prefix != "" && (seg == "" || seg == "." || seg == "..") {
			return "", "", fmt.Errorf("objects: %q has an invalid prefix", root)
		}
	}
	return u.Host, prefix, nil
}

// newS3BackendFromEnv builds the client from the default AWS chain: the ECS
// task role supplies credentials and AWS_REGION supplies the region.
func newS3BackendFromEnv(ctx context.Context, root string) (*s3Backend, error) {
	bucket, prefix, err := parseS3Root(root)
	if err != nil {
		return nil, err
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("objects: load aws config: %w", err)
	}
	if cfg.Region == "" {
		return nil, errors.New("objects: an s3 object store needs AWS_REGION")
	}
	return newS3Backend(s3.NewFromConfig(cfg), bucket, prefix), nil
}

func newS3Backend(client s3API, bucket, prefix string) *s3Backend {
	return &s3Backend{client: client, bucket: bucket, prefix: prefix}
}

func (b *s3Backend) key(name string) string {
	if b.prefix == "" {
		return name
	}
	return b.prefix + "/" + name
}

func (b *s3Backend) put(ctx context.Context, name string, data []byte) error {
	if !validSHA256Hex(name) && !validOwnedObjectKey(name) {
		return errors.New("objects: invalid S3 object key")
	}
	sum := sha256.Sum256(data)
	var lastErr error
	for range s3PutAttempts {
		_, err := b.client.PutObject(ctx, &s3.PutObjectInput{
			Bucket:        aws.String(b.bucket),
			Key:           aws.String(b.key(name)),
			Body:          bytes.NewReader(data),
			ContentLength: aws.Int64(int64(len(data))),
			ContentType:   aws.String("application/octet-stream"),
			// The name is the sha256 of data, so S3 re-verifies the body.
			ChecksumSHA256: aws.String(base64.StdEncoding.EncodeToString(sum[:])),
			// Content addressing makes an existing key the same bytes, so a
			// 412 here means the blob is already stored.
			IfNoneMatch: aws.String("*"),
		})
		switch status := s3Status(err); {
		case err == nil:
			return nil
		case status == http.StatusPreconditionFailed:
			// Frozen legacy digest writes treat a content-addressed 412 as
			// success. Owned v2 keys require byte-for-byte collision proof.
			if !validOwnedObjectKey(name) {
				return nil
			}
			// Conditional retries must verify the existing bytes; a key collision
			// is never treated as successful immutable storage by assumption.
			existing, getErr := b.get(ctx, name)
			if getErr == nil && bytes.Equal(existing, data) {
				return nil
			}
			if getErr != nil {
				return fmt.Errorf("verify existing object after conditional put: %w", getErr)
			}
			return errors.New("objects: immutable S3 key contains different bytes")
		case status == http.StatusConflict:
			lastErr = err
			continue
		default:
			return fmt.Errorf("write object: %w", err)
		}
	}
	return fmt.Errorf("write object: %w", lastErr)
}

func (b *s3Backend) get(ctx context.Context, name string) ([]byte, error) {
	return b.getLimit(ctx, name, int64(^uint64(0)>>1))
}

func (b *s3Backend) getLimit(ctx context.Context, name string, limit int64) ([]byte, error) {
	if !validSHA256Hex(name) && !validOwnedObjectKey(name) {
		return nil, errors.New("objects: invalid S3 object key")
	}
	out, err := b.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(b.key(name)),
	})
	if err != nil {
		var noKey *types.NoSuchKey
		if errors.As(err, &noKey) || s3Status(err) == http.StatusNotFound {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("read object: %w", err)
	}
	// Closing this read-only object stream cannot change the received result.
	defer func() { _ = out.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(out.Body, limit))
	if err != nil {
		return nil, fmt.Errorf("read object: %w", err)
	}
	return data, nil
}

func (b *s3Backend) delete(ctx context.Context, name string) error {
	if !validOwnedObjectKey(name) {
		return errors.New("objects: invalid owned S3 key")
	}
	client, ok := b.client.(s3DeleteAPI)
	if !ok {
		return errors.New("objects: S3 client has no delete operation")
	}
	_, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(b.bucket), Key: aws.String(b.key(name))})
	if err != nil {
		return fmt.Errorf("delete owned object: %w", err)
	}
	return nil
}

// s3Status returns the HTTP status of a failed S3 call, or 0.
func s3Status(err error) int {
	var re *smithyhttp.ResponseError
	if errors.As(err, &re) {
		return re.HTTPStatusCode()
	}
	return 0
}
