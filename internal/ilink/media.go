package ilink

import (
	"bytes"
	"crypto/aes"
	"encoding/hex"
	"encoding/base64"
	"fmt"
)

// MediaProcessor handles media file encryption/decryption for iLink CDN.
type MediaProcessor struct{}

// NewMediaProcessor creates a new media processor.
func NewMediaProcessor() *MediaProcessor {
	return &MediaProcessor{}
}

// DecryptKey decodes AES key from various formats.
// Supports three formats:
// 1. Direct hex (32 chars) → hex decode → 16 bytes
// 2. base64(raw 16 bytes) → base64 decode → 16 bytes
// 3. base64(hex string) → base64 decode → hex decode → 16 bytes
func (mp *MediaProcessor) DecryptKey(encodedKey string) ([]byte, error) {
	// Try format 1: direct hex (32 hex chars)
	if len(encodedKey) == 32 {
		if key, err := hex.DecodeString(encodedKey); err == nil && len(key) == 16 {
			return key, nil
		}
	}

	// Try base64 decode
	decoded, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode key: %v", err)
	}

	// Format 2: base64(raw 16 bytes)
	if len(decoded) == 16 {
		return decoded, nil
	}

	// Format 3: base64(hex string) → 32 bytes
	if len(decoded) == 32 {
		if key, err := hex.DecodeString(string(decoded)); err == nil && len(key) == 16 {
			return key, nil
		}
	}

	return nil, fmt.Errorf("unknown key format")
}

// EncryptAES128ECB encrypts data using AES-128-ECB with PKCS7 padding.
func (mp *MediaProcessor) EncryptAES128ECB(plaintext []byte, key []byte) ([]byte, error) {
	if len(key) != 16 {
		return nil, fmt.Errorf("key must be 16 bytes")
	}

	// Add PKCS7 padding
	plaintext = pkcs7Pad(plaintext, aes.BlockSize)

	// Create cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// ECB mode encryption
	ciphertext := make([]byte, len(plaintext))
	for bs, be := 0, aes.BlockSize; bs < len(plaintext); bs, be = bs+aes.BlockSize, be+aes.BlockSize {
		block.Encrypt(ciphertext[bs:be], plaintext[bs:be])
	}

	return ciphertext, nil
}

// DecryptAES128ECB decrypts data using AES-128-ECB with PKCS7 unpadding.
func (mp *MediaProcessor) DecryptAES128ECB(ciphertext []byte, key []byte) ([]byte, error) {
	if len(key) != 16 {
		return nil, fmt.Errorf("key must be 16 bytes")
	}

	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext is not a multiple of the block size")
	}

	// Create cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// ECB mode decryption
	plaintext := make([]byte, len(ciphertext))
	for bs, be := 0, aes.BlockSize; bs < len(ciphertext); bs, be = bs+aes.BlockSize, be+aes.BlockSize {
		block.Decrypt(plaintext[bs:be], ciphertext[bs:be])
	}

	// Remove PKCS7 padding
	return pkcs7Unpad(plaintext)
}

// pkcs7Pad adds PKCS7 padding to data.
func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padtext...)
}

// pkcs7Unpad removes PKCS7 padding from data.
func pkcs7Unpad(data []byte) ([]byte, error) {
	length := len(data)
	if length == 0 {
		return nil, fmt.Errorf("empty data")
	}

	padding := int(data[length-1])
	if padding > length {
		return nil, fmt.Errorf("invalid padding")
	}

	return data[:length-padding], nil
}
