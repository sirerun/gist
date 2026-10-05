package storage

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
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
	raw, err := hex.DecodeString(name)
	if err != nil {
		return fmt.Errorf("objects: decode digest: %w", err)
	}
	var lastErr error
	for range s3PutAttempts {
		_, err = b.client.PutObject(ctx, &s3.PutObjectInput{
			Bucket:        aws.String(b.bucket),
			Key:           aws.String(b.key(name)),
			Body:          bytes.NewReader(data),
			ContentLength: aws.Int64(int64(len(data))),
			ContentType:   aws.String("application/octet-stream"),
			// The name is the sha256 of data, so S3 re-verifies the body.
			ChecksumSHA256: aws.String(base64.StdEncoding.EncodeToString(raw)),
			// Content addressing makes an existing key the same bytes, so a
			// 412 here means the blob is already stored.
			IfNoneMatch: aws.String("*"),
		})
		switch status := s3Status(err); {
		case err == nil, status == http.StatusPreconditionFailed:
			return nil
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
	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, fmt.Errorf("read object: %w", err)
	}
	return data, nil
}

// s3Status returns the HTTP status of a failed S3 call, or 0.
func s3Status(err error) int {
	var re *smithyhttp.ResponseError
	if errors.As(err, &re) {
		return re.HTTPStatusCode()
	}
	return 0
}
