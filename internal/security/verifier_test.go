package security

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestNewModelVerifier_Disabled(t *testing.T) {
	v, err := NewModelVerifier(false, "")
	if err != nil {
		t.Fatalf("expected no error for disabled verifier, got: %v", err)
	}
	if v.IsEnabled() {
		t.Fatal("expected verifier to be disabled")
	}
}

func TestNewModelVerifier_EnabledMissingFile(t *testing.T) {
	_, err := NewModelVerifier(true, "/nonexistent/path/key.pem")
	if err == nil {
		t.Fatal("expected error for missing PEM file")
	}
}

func TestNewModelVerifier_EnabledInvalidPEM(t *testing.T) {
	tmpDir := t.TempDir()
	pemPath := filepath.Join(tmpDir, "bad.pem")
	if err := os.WriteFile(pemPath, []byte("not a pem file"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := NewModelVerifier(true, pemPath)
	if err == nil {
		t.Fatal("expected error for invalid PEM data")
	}
}

func TestNewModelVerifier_EnabledValidKey(t *testing.T) {
	pemPath := writeTestPublicKey(t)

	v, err := NewModelVerifier(true, pemPath)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !v.IsEnabled() {
		t.Fatal("expected verifier to be enabled")
	}
}

func TestVerifyChecksum_Correct(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "testdata.bin")
	content := []byte("hello world model data")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	h := sha256.Sum256(content)
	expected := hex.EncodeToString(h[:])

	v := &ModelVerifier{enabled: true}
	if err := v.VerifyChecksum(testFile, expected); err != nil {
		t.Fatalf("expected checksum to pass, got: %v", err)
	}
}

func TestVerifyChecksum_Incorrect(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "testdata.bin")
	content := []byte("hello world model data")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	v := &ModelVerifier{enabled: true}
	err := v.VerifyChecksum(testFile, "0000000000000000000000000000000000000000000000000000000000000000")
	if err == nil {
		t.Fatal("expected checksum mismatch error")
	}
}

func TestVerifyChecksum_DisabledAlwaysPasses(t *testing.T) {
	v := &ModelVerifier{enabled: false}
	// Should pass even with nonsense arguments when disabled
	if err := v.VerifyChecksum("/nonexistent/file", "badhash"); err != nil {
		t.Fatalf("expected disabled verifier to pass, got: %v", err)
	}
}

func TestVerifyChecksum_FileNotFound(t *testing.T) {
	v := &ModelVerifier{enabled: true}
	err := v.VerifyChecksum("/nonexistent/file.bin", "abc123")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestVerifySignature_Valid(t *testing.T) {
	privKey, pemPath := generateTestKeyPair(t)

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "model.bin")
	content := []byte("test model binary data for signature verification")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	// Sign the file content
	h := sha256.Sum256(content)
	sig, err := rsa.SignPSS(rand.Reader, privKey, crypto.SHA256, h[:], &rsa.PSSOptions{
		SaltLength: rsa.PSSSaltLengthAuto,
		Hash:       crypto.SHA256,
	})
	if err != nil {
		t.Fatalf("failed to sign: %v", err)
	}
	sigBase64 := base64.StdEncoding.EncodeToString(sig)

	v, err := NewModelVerifier(true, pemPath)
	if err != nil {
		t.Fatalf("failed to create verifier: %v", err)
	}

	if err := v.VerifySignature(testFile, sigBase64); err != nil {
		t.Fatalf("expected signature verification to pass, got: %v", err)
	}
}

func TestVerifySignature_Invalid(t *testing.T) {
	_, pemPath := generateTestKeyPair(t)

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "model.bin")
	content := []byte("test model binary data")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	// Use a garbage signature
	badSig := base64.StdEncoding.EncodeToString([]byte("this is not a valid signature at all"))

	v, err := NewModelVerifier(true, pemPath)
	if err != nil {
		t.Fatalf("failed to create verifier: %v", err)
	}

	if err := v.VerifySignature(testFile, badSig); err == nil {
		t.Fatal("expected signature verification to fail for bad signature")
	}
}

func TestVerifySignature_DisabledAlwaysPasses(t *testing.T) {
	v := &ModelVerifier{enabled: false}
	if err := v.VerifySignature("/nonexistent/file", "badsig"); err != nil {
		t.Fatalf("expected disabled verifier to pass, got: %v", err)
	}
}

func TestVerifySignature_InvalidBase64(t *testing.T) {
	pemPath := writeTestPublicKey(t)

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "model.bin")
	if err := os.WriteFile(testFile, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	v, err := NewModelVerifier(true, pemPath)
	if err != nil {
		t.Fatalf("failed to create verifier: %v", err)
	}

	err = v.VerifySignature(testFile, "%%%not-base64%%%")
	if err == nil {
		t.Fatal("expected error for invalid base64 signature")
	}
}

// generateTestKeyPair generates an RSA key pair, writes the public key PEM to a temp file,
// and returns the private key and path to the public key PEM file.
func generateTestKeyPair(t *testing.T) (*rsa.PrivateKey, string) {
	t.Helper()
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	pubKeyBytes, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	if err != nil {
		t.Fatalf("failed to marshal public key: %v", err)
	}

	pemBlock := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyBytes,
	}

	tmpDir := t.TempDir()
	pemPath := filepath.Join(tmpDir, "public.pem")
	pemFile, err := os.Create(pemPath)
	if err != nil {
		t.Fatalf("failed to create PEM file: %v", err)
	}
	defer pemFile.Close()

	if err := pem.Encode(pemFile, pemBlock); err != nil {
		t.Fatalf("failed to encode PEM: %v", err)
	}

	return privKey, pemPath
}

// writeTestPublicKey generates a key pair and writes only the public key PEM, returning its path.
func writeTestPublicKey(t *testing.T) string {
	t.Helper()
	_, pemPath := generateTestKeyPair(t)
	return pemPath
}
