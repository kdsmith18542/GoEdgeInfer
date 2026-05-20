package security

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
)

// ModelVerifier provides model integrity verification using SHA256 checksums
// and RSA-PSS digital signatures.
type ModelVerifier struct {
	enabled   bool
	publicKey *rsa.PublicKey
}

// NewModelVerifier creates a new ModelVerifier instance.
// If enabled is false, returns a verifier that always passes (no-op).
// If enabled is true, reads the PEM file at publicKeyPemPath and parses the RSA public key.
func NewModelVerifier(enabled bool, publicKeyPemPath string) (*ModelVerifier, error) {
	if !enabled {
		return &ModelVerifier{enabled: false}, nil
	}

	pemData, err := os.ReadFile(publicKeyPemPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key PEM file: %w", err)
	}

	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, errors.New("failed to decode PEM block from public key file")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("public key is not an RSA key")
	}

	return &ModelVerifier{
		enabled:   true,
		publicKey: rsaPub,
	}, nil
}

// IsEnabled returns whether verification is enabled.
func (v *ModelVerifier) IsEnabled() bool {
	return v.enabled
}

// VerifyChecksum reads the file at filePath, computes its SHA256 hash,
// and compares it to the expectedSHA256 hex string.
func (v *ModelVerifier) VerifyChecksum(filePath string, expectedSHA256 string) error {
	if !v.enabled {
		return nil
	}

	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file for checksum verification: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("failed to read file for checksum verification: %w", err)
	}

	actual := hex.EncodeToString(h.Sum(nil))
	if actual != expectedSHA256 {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedSHA256, actual)
	}

	return nil
}

// VerifySignature reads the file at filePath, computes its SHA256 hash,
// decodes the base64-encoded signature, and verifies it using the RSA public key
// with PSS padding.
func (v *ModelVerifier) VerifySignature(filePath string, signatureBase64 string) error {
	if !v.enabled {
		return nil
	}

	if v.publicKey == nil {
		return errors.New("no public key loaded for signature verification")
	}

	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file for signature verification: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("failed to read file for signature verification: %w", err)
	}
	digest := h.Sum(nil)

	sig, err := base64.StdEncoding.DecodeString(signatureBase64)
	if err != nil {
		return fmt.Errorf("failed to decode base64 signature: %w", err)
	}

	opts := &rsa.PSSOptions{
		SaltLength: rsa.PSSSaltLengthAuto,
		Hash:       crypto.SHA256,
	}

	if err := rsa.VerifyPSS(v.publicKey, crypto.SHA256, digest, sig, opts); err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}

	return nil
}
