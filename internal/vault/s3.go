package vault

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"bt-go/internal/bt"
)

// s3API is the subset of s3.Client methods used by S3Vault.
// Defined here (not in the s3 package) for testability.
type s3API interface {
	HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	HeadBucket(ctx context.Context, params *s3.HeadBucketInput, optFns ...func(*s3.Options)) (*s3.HeadBucketOutput, error)
}

// s3UploadAPI is the subset of manager.Uploader methods used by S3Vault.
// Defined here for testability.
type s3UploadAPI interface {
	Upload(ctx context.Context, input *s3.PutObjectInput, opts ...func(*manager.Uploader)) (*manager.UploadOutput, error)
}

// S3Vault is an S3-backed implementation of the Vault interface.
// Content objects are stored under contentPrefix; metadata objects under metadataPrefix.
// Object key layout:
//
//	{contentPrefix}/{checksum}                   → content object
//	{metadataPrefix}/{hostID}/{name}             → metadata object
//	{metadataPrefix}/{hostID}/{name}.version     → version marker (body = int64 string)
type S3Vault struct {
	name           string
	bucket         string
	contentPrefix  string
	metadataPrefix string
	client         s3API
	uploader       s3UploadAPI
}

// NewS3Vault creates a new S3Vault with explicit AWS credentials.
// Credentials are scoped to this vault and do not affect other AWS SDK usage on the machine.
func NewS3Vault(name, bucket, contentPrefix, metadataPrefix, region, accessKeyID, secretAccessKey string) (*S3Vault, error) {
	creds := credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(creds),
	)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}
	client := s3.NewFromConfig(cfg)
	uploader := manager.NewUploader(client)
	return &S3Vault{
		name:           name,
		bucket:         bucket,
		contentPrefix:  contentPrefix,
		metadataPrefix: metadataPrefix,
		client:         client,
		uploader:       uploader,
	}, nil
}

// PutContent stores content identified by its checksum.
// If the content already exists in the vault, the reader is discarded and no upload is performed.
func (v *S3Vault) PutContent(checksum string, r io.Reader, size int64) error {
	ctx := context.Background()
	key := s3Key(v.contentPrefix, checksum)

	_, err := v.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(v.bucket),
		Key:    aws.String(key),
	})
	if err == nil {
		// Content already exists; discard reader to satisfy caller expectations.
		_, _ = io.Copy(io.Discard, r)
		return nil
	}
	if !isNotFound(err) {
		return fmt.Errorf("checking content %s: %w", checksum, err)
	}

	_, err = v.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(v.bucket),
		Key:           aws.String(key),
		Body:          r,
		ContentLength: aws.Int64(size),
	})
	if err != nil {
		return fmt.Errorf("uploading content %s: %w", checksum, err)
	}
	return nil
}

// GetContent retrieves content by checksum and writes it to w.
func (v *S3Vault) GetContent(checksum string, w io.Writer) error {
	ctx := context.Background()
	key := s3Key(v.contentPrefix, checksum)

	out, err := v.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(v.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return fmt.Errorf("content not found: %s", checksum)
		}
		return fmt.Errorf("getting content %s: %w", checksum, err)
	}
	defer out.Body.Close()

	if _, err := io.Copy(w, out.Body); err != nil {
		return fmt.Errorf("reading content %s: %w", checksum, err)
	}
	return nil
}

// PutMetadata stores a named metadata item for a specific host with a version marker.
func (v *S3Vault) PutMetadata(hostID string, name string, r io.Reader, size int64, version int64) error {
	ctx := context.Background()
	dataKey := s3Key(v.metadataPrefix, hostID, name)
	versionKey := dataKey + ".version"

	_, err := v.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(v.bucket),
		Key:           aws.String(dataKey),
		Body:          r,
		ContentLength: aws.Int64(size),
	})
	if err != nil {
		return fmt.Errorf("putting metadata %s/%s: %w", hostID, name, err)
	}

	versionStr := strconv.FormatInt(version, 10)
	_, err = v.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(v.bucket),
		Key:           aws.String(versionKey),
		Body:          strings.NewReader(versionStr),
		ContentLength: aws.Int64(int64(len(versionStr))),
	})
	if err != nil {
		return fmt.Errorf("putting version for metadata %s/%s: %w", hostID, name, err)
	}
	return nil
}

// GetMetadata retrieves a named metadata item for a specific host and writes it to w.
func (v *S3Vault) GetMetadata(hostID string, name string, w io.Writer) error {
	ctx := context.Background()
	key := s3Key(v.metadataPrefix, hostID, name)

	out, err := v.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(v.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return fmt.Errorf("metadata %q not found for host: %s", name, hostID)
		}
		return fmt.Errorf("getting metadata %s/%s: %w", hostID, name, err)
	}
	defer out.Body.Close()

	if _, err := io.Copy(w, out.Body); err != nil {
		return fmt.Errorf("reading metadata %s/%s: %w", hostID, name, err)
	}
	return nil
}

// GetMetadataVersion returns the stored version for a named metadata item.
// Returns 0 if no version has been stored yet.
func (v *S3Vault) GetMetadataVersion(hostID string, name string) (int64, error) {
	ctx := context.Background()
	key := s3Key(v.metadataPrefix, hostID, name) + ".version"

	out, err := v.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(v.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("getting metadata version %s/%s: %w", hostID, name, err)
	}
	defer out.Body.Close()

	data, err := io.ReadAll(out.Body)
	if err != nil {
		return 0, fmt.Errorf("reading metadata version %s/%s: %w", hostID, name, err)
	}
	version, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing metadata version %s/%s: %w", hostID, name, err)
	}
	return version, nil
}

// ValidateSetup verifies that the bucket exists and credentials are valid.
func (v *S3Vault) ValidateSetup() error {
	_, err := v.client.HeadBucket(context.Background(), &s3.HeadBucketInput{
		Bucket: aws.String(v.bucket),
	})
	if err != nil {
		return fmt.Errorf("validating S3 vault setup (bucket %q): %w", v.bucket, err)
	}
	return nil
}

// s3Key joins non-empty path components with "/" to form an S3 object key.
func s3Key(parts ...string) string {
	var nonEmpty []string
	for _, p := range parts {
		if p != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	return strings.Join(nonEmpty, "/")
}

// isNotFound returns true if the error indicates the requested S3 object does not exist.
func isNotFound(err error) bool {
	var noKey *types.NoSuchKey
	if errors.As(err, &noKey) {
		return true
	}
	var notFound *types.NotFound
	if errors.As(err, &notFound) {
		return true
	}
	return false
}

// Compile-time check that S3Vault implements bt.Vault interface.
var _ bt.Vault = (*S3Vault)(nil)
