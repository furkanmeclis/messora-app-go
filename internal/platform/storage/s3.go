package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	tmtypes "github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// S3Options configure the S3 backend.
type S3Options struct {
	Region string
	// Endpoint is set for S3-compatible services (Cloudflare R2, MinIO);
	// empty means AWS.
	Endpoint string
	// ForcePathStyle addresses buckets as <endpoint>/<bucket> (MinIO).
	ForcePathStyle bool
	// AccessKeyID and SecretAccessKey are optional static credentials;
	// without them the default chain is used (environment, shared files,
	// the EC2 instance role or the ECS task role).
	AccessKeyID     string
	SecretAccessKey string
	// SSE is the server-side encryption ("AES256" by default; "none"
	// disables it for services that refuse the header).
	SSE string
	// PartSize of multipart uploads (default 16 MiB).
	PartSize int64
	// HTTPClient overrides the transport (tests).
	HTTPClient *http.Client
}

// S3 is one S3 bucket.
type S3 struct {
	client  *s3.Client
	presign *s3.PresignClient
	upload  *transfermanager.Client
	bucket  string
	class   Class
	public  string // base URL of public objects
	sse     tmtypes.ServerSideEncryption
}

// NewS3Client builds an S3 client from o.
func NewS3Client(ctx context.Context, o S3Options) (*s3.Client, error) {
	if o.Region == "" {
		return nil, errors.New("storage: S3_REGION is required")
	}
	loaders := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(o.Region)}
	if o.AccessKeyID != "" || o.SecretAccessKey != "" {
		if o.AccessKeyID == "" || o.SecretAccessKey == "" {
			return nil, errors.New("storage: S3_ACCESS_KEY_ID and S3_SECRET_ACCESS_KEY must be set together")
		}
		loaders = append(loaders, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(o.AccessKeyID, o.SecretAccessKey, "")))
	}
	if o.HTTPClient != nil {
		loaders = append(loaders, awsconfig.WithHTTPClient(o.HTTPClient))
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, loaders...)
	if err != nil {
		return nil, fmt.Errorf("storage: load AWS configuration: %w", err)
	}
	return s3.NewFromConfig(cfg, func(so *s3.Options) {
		if o.Endpoint != "" {
			so.BaseEndpoint = aws.String(o.Endpoint)
			// S3-compatible services differ in their support for the
			// newer default checksums; send them only when required.
			so.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
			so.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
		}
		so.UsePathStyle = o.ForcePathStyle
	}), nil
}

// NewS3 returns a store for bucket. publicBase is the base URL of public
// objects (public class only).
func NewS3(client *s3.Client, bucket string, class Class, o S3Options, publicBase string) (*S3, error) {
	if bucket == "" {
		return nil, fmt.Errorf("storage: the %s bucket name is empty", class)
	}
	part := o.PartSize
	if part <= 0 {
		part = 16 << 20
	}
	s := &S3{client: client, presign: s3.NewPresignClient(client), bucket: bucket, class: class,
		public: strings.TrimRight(publicBase, "/")}
	s.upload = transfermanager.New(client, func(t *transfermanager.Options) {
		t.PartSizeBytes = part
		t.MultipartUploadThreshold = part
		t.Concurrency = 3
		// Abort a failed multipart upload even when the request's context
		// was cancelled, so no orphaned parts are billed.
		t.FailTimeout = 30 * time.Second
		if o.Endpoint != "" {
			t.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		}
	})
	switch strings.ToLower(o.SSE) {
	case "", "aes256":
		s.sse = tmtypes.ServerSideEncryptionAes256
	case "none":
	default:
		return nil, fmt.Errorf("storage: S3_SSE must be AES256 or none (got %q)", o.SSE)
	}
	return s, nil
}

// Bucket returns the bucket name.
func (s *S3) Bucket() string { return s.bucket }

// Put implements Store. Bodies of unknown size and large ones go up as a
// multipart upload (in PartSize chunks); a failed upload is aborted.
func (s *S3) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string, opts PutOptions) error {
	if err := ValidateKey(key); err != nil {
		return err
	}
	in := &transfermanager.UploadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key), Body: r,
		ServerSideEncryption: s.sse}
	if contentType != "" {
		in.ContentType = aws.String(contentType)
	}
	if opts.CacheControl != "" {
		in.CacheControl = aws.String(opts.CacheControl)
	}
	if opts.ContentDisposition != "" {
		in.ContentDisposition = aws.String(opts.ContentDisposition)
	}
	if size >= 0 {
		in.ContentLength = aws.Int64(size)
	}
	if _, err := s.upload.UploadObject(ctx, in); err != nil {
		return fmt.Errorf("storage: put %s: %w", key, err)
	}
	return nil
}

func isNotFound(err error) bool {
	var nsk *types.NoSuchKey
	var nf *types.NotFound
	if errors.As(err, &nsk) || errors.As(err, &nf) {
		return true
	}
	var api smithy.APIError
	return errors.As(err, &api) && (api.ErrorCode() == "NotFound" || api.ErrorCode() == "NoSuchKey")
}

// Get implements Store.
func (s *S3) Get(ctx context.Context, key string) (io.ReadCloser, Info, error) {
	if err := ValidateKey(key); err != nil {
		return nil, Info{}, err
	}
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if isNotFound(err) {
		return nil, Info{}, ErrNotFound
	}
	if err != nil {
		return nil, Info{}, fmt.Errorf("storage: get %s: %w", key, err)
	}
	return out.Body, Info{Key: key, Size: aws.ToInt64(out.ContentLength), ContentType: aws.ToString(out.ContentType),
		ModTime: aws.ToTime(out.LastModified), ETag: aws.ToString(out.ETag)}, nil
}

// getRange implements rangeGetter; info.Size is the length of the range.
func (s *S3) getRange(ctx context.Context, key, rng string) (io.ReadCloser, Info, string, error) {
	if err := ValidateKey(key); err != nil {
		return nil, Info{}, "", err
	}
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key), Range: aws.String(rng)})
	if isNotFound(err) {
		return nil, Info{}, "", ErrNotFound
	}
	var api smithy.APIError
	if errors.As(err, &api) && api.ErrorCode() == "InvalidRange" {
		return nil, Info{}, "", errRangeNotSatisfiable
	}
	if err != nil {
		return nil, Info{}, "", fmt.Errorf("storage: get %s: %w", key, err)
	}
	return out.Body, Info{Key: key, Size: aws.ToInt64(out.ContentLength), ContentType: aws.ToString(out.ContentType),
		ModTime: aws.ToTime(out.LastModified), ETag: aws.ToString(out.ETag)}, aws.ToString(out.ContentRange), nil
}

// Stat implements Store.
func (s *S3) Stat(ctx context.Context, key string) (Info, error) {
	if err := ValidateKey(key); err != nil {
		return Info{}, err
	}
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if isNotFound(err) {
		return Info{}, ErrNotFound
	}
	if err != nil {
		return Info{}, fmt.Errorf("storage: stat %s: %w", key, err)
	}
	return Info{Key: key, Size: aws.ToInt64(out.ContentLength), ContentType: aws.ToString(out.ContentType),
		ModTime: aws.ToTime(out.LastModified), ETag: aws.ToString(out.ETag)}, nil
}

// Delete implements Store.
func (s *S3) Delete(ctx context.Context, key string) error {
	if err := ValidateKey(key); err != nil {
		return err
	}
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil && !isNotFound(err) {
		return fmt.Errorf("storage: delete %s: %w", key, err)
	}
	return nil
}

// PresignGet implements Store.
func (s *S3) PresignGet(ctx context.Context, key string, ttl time.Duration, opts PresignOptions) (string, error) {
	if err := ValidateKey(key); err != nil {
		return "", err
	}
	if ttl <= 0 {
		return "", errors.New("storage: presign ttl must be positive")
	}
	in := &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)}
	if opts.ContentType != "" {
		in.ResponseContentType = aws.String(opts.ContentType)
	}
	if cd := opts.header(); cd != "" {
		in.ResponseContentDisposition = aws.String(cd)
	}
	req, err := s.presign.PresignGetObject(ctx, in, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("storage: presign %s: %w", key, err)
	}
	return req.URL, nil
}

// PublicURL implements Store.
func (s *S3) PublicURL(key string) (string, bool) {
	if s.class != Public || ValidateKey(key) != nil {
		return "", false
	}
	return s.public + "/" + escapeKey(key), true
}

// Ping implements Store (HeadBucket).
func (s *S3) Ping(ctx context.Context) error {
	if _, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.bucket)}); err != nil {
		return fmt.Errorf("bucket %s: %w", s.bucket, err)
	}
	return nil
}
