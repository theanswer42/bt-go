package encryption

import (
	"fmt"

	"bt-go/internal/bt"
	"bt-go/internal/config"
)

// NewEncryptorFromConfig creates an Encryptor based on the configuration type.
func NewEncryptorFromConfig(cfg config.EncryptionConfig) (bt.Encryptor, error) {
	switch cfg.Type {
	case "age", "":
		return NewAgeEncryptor(cfg), nil
	case "test":
		return NewTestEncryptor(), nil
	default:
		return nil, fmt.Errorf("unknown encryption type: %q", cfg.Type)
	}
}
