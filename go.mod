module bt-go

go 1.25.6

require (
	filippo.io/age v1.3.1
	github.com/BurntSushi/toml v1.6.0
	github.com/aws/aws-sdk-go-v2 v1.41.4 // AWS SDK core — S3Vault backend
	github.com/aws/aws-sdk-go-v2/config v1.32.12 // AWS config loading for S3Vault
	github.com/aws/aws-sdk-go-v2/credentials v1.19.12 // Static credentials provider for S3Vault
	github.com/aws/aws-sdk-go-v2/feature/s3/manager v1.22.4 // Multipart upload manager for S3Vault
	github.com/aws/aws-sdk-go-v2/service/s3 v1.96.4 // S3 client for S3Vault
	github.com/golang-migrate/migrate/v4 v4.19.1
	github.com/google/uuid v1.6.0
	github.com/mattn/go-sqlite3 v1.14.34
	github.com/spf13/cobra v1.10.2
	golang.org/x/term v0.40.0
)

require (
	filippo.io/hpke v0.4.0 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.6 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.20 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.20 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.20 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.6 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.20 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.9.11 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.20 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.19.19 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.0.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.30.13 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.35.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.41.9 // indirect
	github.com/aws/smithy-go v1.24.2 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	golang.org/x/crypto v0.45.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
)
