package sprig

import (
	"crypto/rand"
	"encoding/base64"
	"github.com/google/uuid"
)

func randBytes(count int) (string, error) {
	buf := make([]byte, count)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf), nil
}

// uuidv4 provides a safe and secure UUID v4 implementation
func uuidv4() string {
	return uuid.New().String()
}
