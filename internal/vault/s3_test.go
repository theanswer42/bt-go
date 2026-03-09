package vault

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// mockS3Client implements s3API for testing.
type mockS3Client struct {
	headObjectFn func(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	getObjectFn  func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	putObjectFn  func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	headBucketFn func(ctx context.Context, params *s3.HeadBucketInput, optFns ...func(*s3.Options)) (*s3.HeadBucketOutput, error)
}

func (m *mockS3Client) HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	if m.headObjectFn == nil {
		panic("unexpected call to HeadObject")
	}
	return m.headObjectFn(ctx, params, optFns...)
}

func (m *mockS3Client) GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	if m.getObjectFn == nil {
		panic("unexpected call to GetObject")
	}
	return m.getObjectFn(ctx, params, optFns...)
}

func (m *mockS3Client) PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	if m.putObjectFn == nil {
		panic("unexpected call to PutObject")
	}
	return m.putObjectFn(ctx, params, optFns...)
}

func (m *mockS3Client) HeadBucket(ctx context.Context, params *s3.HeadBucketInput, optFns ...func(*s3.Options)) (*s3.HeadBucketOutput, error) {
	if m.headBucketFn == nil {
		panic("unexpected call to HeadBucket")
	}
	return m.headBucketFn(ctx, params, optFns...)
}

// mockUploader implements s3UploadAPI for testing.
type mockUploader struct {
	uploadFn func(ctx context.Context, input *s3.PutObjectInput, opts ...func(*manager.Uploader)) (*manager.UploadOutput, error)
}

func (m *mockUploader) Upload(ctx context.Context, input *s3.PutObjectInput, opts ...func(*manager.Uploader)) (*manager.UploadOutput, error) {
	if m.uploadFn == nil {
		panic("unexpected call to Upload")
	}
	return m.uploadFn(ctx, input, opts...)
}

// newTestVault creates an S3Vault with injected mocks.
func newTestVault(cl *mockS3Client, ul *mockUploader) *S3Vault {
	return &S3Vault{
		name:           "test",
		bucket:         "test-bucket",
		contentPrefix:  "content",
		metadataPrefix: "metadata",
		client:         cl,
		uploader:       ul,
	}
}

func TestS3Vault_PutContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		checksum string
		data     string
		size     int64
		setup    func(*mockS3Client, *mockUploader)
		wantErr  bool
	}{
		{
			name:     "uploads new content",
			checksum: "abc123",
			data:     "hello world",
			size:     11,
			setup: func(cl *mockS3Client, ul *mockUploader) {
				cl.headObjectFn = func(_ context.Context, _ *s3.HeadObjectInput, _ ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
					return nil, &types.NotFound{}
				}
				ul.uploadFn = func(_ context.Context, input *s3.PutObjectInput, _ ...func(*manager.Uploader)) (*manager.UploadOutput, error) {
					if *input.Key != "content/abc123" {
						return nil, fmt.Errorf("unexpected key: %s", *input.Key)
					}
					return &manager.UploadOutput{}, nil
				}
			},
		},
		{
			name:     "skips existing content",
			checksum: "abc123",
			data:     "hello world",
			size:     11,
			setup: func(cl *mockS3Client, ul *mockUploader) {
				cl.headObjectFn = func(_ context.Context, _ *s3.HeadObjectInput, _ ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
					return &s3.HeadObjectOutput{}, nil
				}
				// uploader must not be called; nil fn will panic if it is
			},
		},
		{
			name:     "propagates HeadObject error",
			checksum: "abc123",
			data:     "hello world",
			size:     11,
			setup: func(cl *mockS3Client, _ *mockUploader) {
				cl.headObjectFn = func(_ context.Context, _ *s3.HeadObjectInput, _ ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
					return nil, fmt.Errorf("internal server error")
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cl := &mockS3Client{}
			ul := &mockUploader{}
			tt.setup(cl, ul)
			v := newTestVault(cl, ul)

			err := v.PutContent(tt.checksum, strings.NewReader(tt.data), tt.size)
			if (err != nil) != tt.wantErr {
				t.Errorf("PutContent() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestS3Vault_GetContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		checksum string
		getObj   func(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error)
		wantData string
		wantErr  bool
		wantMsg  string
	}{
		{
			name:     "returns content",
			checksum: "abc123",
			getObj: func(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				return &s3.GetObjectOutput{
					Body: io.NopCloser(strings.NewReader("hello world")),
				}, nil
			},
			wantData: "hello world",
		},
		{
			name:     "returns error for missing content",
			checksum: "missing",
			getObj: func(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				return nil, &types.NoSuchKey{}
			},
			wantErr: true,
			wantMsg: "content not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cl := &mockS3Client{getObjectFn: tt.getObj}
			v := newTestVault(cl, &mockUploader{})

			var buf bytes.Buffer
			err := v.GetContent(tt.checksum, &buf)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetContent() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.wantMsg != "" && !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("GetContent() error = %q, want message containing %q", err.Error(), tt.wantMsg)
			}
			if !tt.wantErr && buf.String() != tt.wantData {
				t.Errorf("GetContent() data = %q, want %q", buf.String(), tt.wantData)
			}
		})
	}
}

func TestS3Vault_PutMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		hostID    string
		mdName    string
		data      string
		version   int64
		putObjErr error
		wantErr   bool
	}{
		{
			name:    "stores data and version objects",
			hostID:  "host-123",
			mdName:  "db",
			data:    "metadata content",
			version: 42,
		},
		{
			name:      "propagates PutObject error",
			hostID:    "host-123",
			mdName:    "db",
			data:      "metadata content",
			version:   1,
			putObjErr: fmt.Errorf("access denied"),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var putKeys []string
			cl := &mockS3Client{
				putObjectFn: func(_ context.Context, params *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
					if tt.putObjErr != nil {
						return nil, tt.putObjErr
					}
					putKeys = append(putKeys, *params.Key)
					return &s3.PutObjectOutput{}, nil
				},
			}
			v := newTestVault(cl, &mockUploader{})

			err := v.PutMetadata(tt.hostID, tt.mdName, strings.NewReader(tt.data), int64(len(tt.data)), tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("PutMetadata() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				wantDataKey := "metadata/" + tt.hostID + "/" + tt.mdName
				wantVersionKey := wantDataKey + ".version"
				if len(putKeys) != 2 {
					t.Fatalf("PutObject called %d times, want 2", len(putKeys))
				}
				if putKeys[0] != wantDataKey {
					t.Errorf("first PutObject key = %q, want %q", putKeys[0], wantDataKey)
				}
				if putKeys[1] != wantVersionKey {
					t.Errorf("second PutObject key = %q, want %q", putKeys[1], wantVersionKey)
				}
			}
		})
	}
}

func TestS3Vault_GetMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		hostID   string
		mdName   string
		getObj   func(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error)
		wantData string
		wantErr  bool
		wantMsg  string
	}{
		{
			name:   "returns metadata",
			hostID: "host-123",
			mdName: "db",
			getObj: func(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				return &s3.GetObjectOutput{
					Body: io.NopCloser(strings.NewReader("db content")),
				}, nil
			},
			wantData: "db content",
		},
		{
			name:   "returns error for missing metadata",
			hostID: "host-123",
			mdName: "db",
			getObj: func(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				return nil, &types.NoSuchKey{}
			},
			wantErr: true,
			wantMsg: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cl := &mockS3Client{getObjectFn: tt.getObj}
			v := newTestVault(cl, &mockUploader{})

			var buf bytes.Buffer
			err := v.GetMetadata(tt.hostID, tt.mdName, &buf)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetMetadata() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.wantMsg != "" && !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("GetMetadata() error = %q, want message containing %q", err.Error(), tt.wantMsg)
			}
			if !tt.wantErr && buf.String() != tt.wantData {
				t.Errorf("GetMetadata() data = %q, want %q", buf.String(), tt.wantData)
			}
		})
	}
}

func TestS3Vault_GetMetadataVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		getObj      func(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error)
		wantVersion int64
		wantErr     bool
	}{
		{
			name: "returns 0 when not found",
			getObj: func(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				return nil, &types.NoSuchKey{}
			},
			wantVersion: 0,
		},
		{
			name: "returns parsed version",
			getObj: func(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
				return &s3.GetObjectOutput{
					Body: io.NopCloser(strings.NewReader("42")),
				}, nil
			},
			wantVersion: 42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cl := &mockS3Client{getObjectFn: tt.getObj}
			v := newTestVault(cl, &mockUploader{})

			got, err := v.GetMetadataVersion("host-123", "db")
			if (err != nil) != tt.wantErr {
				t.Errorf("GetMetadataVersion() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.wantVersion {
				t.Errorf("GetMetadataVersion() = %d, want %d", got, tt.wantVersion)
			}
		})
	}
}

func TestS3Vault_ValidateSetup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		headBucket  func(_ context.Context, _ *s3.HeadBucketInput, _ ...func(*s3.Options)) (*s3.HeadBucketOutput, error)
		wantErr     bool
	}{
		{
			name: "returns nil on success",
			headBucket: func(_ context.Context, _ *s3.HeadBucketInput, _ ...func(*s3.Options)) (*s3.HeadBucketOutput, error) {
				return &s3.HeadBucketOutput{}, nil
			},
		},
		{
			name: "propagates HeadBucket error",
			headBucket: func(_ context.Context, _ *s3.HeadBucketInput, _ ...func(*s3.Options)) (*s3.HeadBucketOutput, error) {
				return nil, fmt.Errorf("access denied")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cl := &mockS3Client{headBucketFn: tt.headBucket}
			v := newTestVault(cl, &mockUploader{})

			err := v.ValidateSetup()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSetup() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestS3Key(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		parts []string
		want  string
	}{
		{name: "joins parts", parts: []string{"content", "abc123"}, want: "content/abc123"},
		{name: "skips empty prefix", parts: []string{"", "abc123"}, want: "abc123"},
		{name: "three parts", parts: []string{"meta", "host-1", "db"}, want: "meta/host-1/db"},
		{name: "all empty", parts: []string{"", ""}, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := s3Key(tt.parts...); got != tt.want {
				t.Errorf("s3Key() = %q, want %q", got, tt.want)
			}
		})
	}
}
