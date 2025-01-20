package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// GenerateJWTSecret generates a cryptographically secure 32-byte random secret
// and returns it as both a byte slice and its hex-encoded string representation.
// If there's an error reading from the crypto/rand source, it returns the error.
func GenerateSecret() (string, error) {
	secret := make([]byte, 32)
	_, err := rand.Read(secret)
	if err != nil {
		return "", fmt.Errorf("failed to generate random secret: %w", err)
	}

	// Convert to hex string for easy storage/configuration
	hexSecret := hex.EncodeToString(secret)

	return hexSecret, nil
}
