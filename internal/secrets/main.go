// Package secrets manages the AES-256-GCM key used to encrypt ESXi root
// passwords at rest, and the encryption and decryption of those passwords.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

// Credits to this person for excellent code: https://www.melvinvivas.com/how-to-encrypt-and-decrypt-data-using-aes/

const (
	keyDir  = "secret"
	keyFile = "secret/secret.key"

	// keySize is 32 bytes for AES-256.
	keySize = 32
)

// Init loads the secret key, generating and persisting a new one if none
// exists. The returned key is hex encoded.
func Init() (string, error) {
	if _, err := os.Stat(keyFile); err == nil {
		logrus.WithFields(logrus.Fields{
			"key": "found existing secret key!",
		}).Info("secrets")

		key, err := os.ReadFile(keyFile)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", keyFile, err)
		}
		return string(key), nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat %s: %w", keyFile, err)
	}

	logrus.WithFields(logrus.Fields{
		"key": "no secrets file has been detected, attempting to create a new one and generate secret key",
	}).Info("secrets")

	if err := os.MkdirAll(keyDir, 0o700); err != nil {
		return "", fmt.Errorf("create %s: %w", keyDir, err)
	}

	b := make([]byte, keySize)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate secret key: %w", err)
	}
	hexkey := hex.EncodeToString(b)

	// Write via a temporary file and rename so an interrupted write cannot
	// leave a truncated key behind — that would render every stored password
	// undecryptable.
	tmp, err := os.CreateTemp(keyDir, ".secret.key-*")
	if err != nil {
		return "", fmt.Errorf("create temporary key file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("chmod temporary key file: %w", err)
	}
	if _, err := tmp.WriteString(hexkey); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("write secret key: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close secret key: %w", err)
	}
	if err := os.Rename(tmpName, filepath.Clean(keyFile)); err != nil {
		return "", fmt.Errorf("persist secret key: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"key":   "secret key persisted to file",
		"bytes": len(hexkey),
	}).Info("secrets")

	return hexkey, nil
}

// newGCM decodes the hex encoded key and returns an AES-GCM cipher for it.
func newGCM(keyString string) (cipher.AEAD, error) {
	key, err := hex.DecodeString(keyString)
	if err != nil {
		return nil, fmt.Errorf("decode secret key: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	// https://en.wikipedia.org/wiki/Galois/Counter_Mode
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	return aesGCM, nil
}

// Encrypt returns the hex encoded AES-256-GCM ciphertext of plaintext, with
// the nonce stored as a prefix of the ciphertext.
func Encrypt(plaintext string, keyString string) (string, error) {
	aesGCM, err := newGCM(keyString)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext), nil
}

// Decrypt reverses Encrypt.
func Decrypt(encrypted string, keyString string) (string, error) {
	aesGCM, err := newGCM(keyString)
	if err != nil {
		return "", err
	}

	enc, err := hex.DecodeString(encrypted)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}

	// Guard the split: a short or empty stored value would otherwise panic on
	// the slice expression rather than report a decryption failure.
	nonceSize := aesGCM.NonceSize()
	if len(enc) < nonceSize+aesGCM.Overhead() {
		return "", fmt.Errorf("ciphertext is too short: %d bytes", len(enc))
	}

	nonce, ciphertext := enc[:nonceSize], enc[nonceSize:]

	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	return string(plaintext), nil
}
