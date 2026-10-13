package shortener

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"strings"
)

const (
	DefaultCodeLength = 6
	base62Alphabet    = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

var (
	ErrEmptyURL   = errors.New("url cannot be empty")
	ErrInvalidURL = errors.New("invalid url format; must be a valid http or https URL")
)

func ValidateAndNormalizeURL(rawURL string) (string, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return "", ErrEmptyURL
	}

	if !strings.HasPrefix(trimmed, "http://") && !strings.HasPrefix(trimmed, "https://") {
		trimmed = "https://" + trimmed
	}

	parsed, err := url.ParseRequestURI(trimmed)
	if err != nil {
		return "", ErrInvalidURL
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", ErrInvalidURL
	}

	if parsed.Host == "" {
		return "", ErrInvalidURL
	}

	if !strings.Contains(parsed.Host, ".") && parsed.Host != "localhost" {
		return "", fmt.Errorf("%w: hostname must be valid", ErrInvalidURL)
	}

	return parsed.String(), nil
}

func GenerateRandomCode(length int) (string, error) {
	if length <= 0 {
		length = DefaultCodeLength
	}

	alphabetLen := big.NewInt(int64(len(base62Alphabet)))
	b := make([]byte, length)

	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, alphabetLen)
		if err != nil {
			return "", err
		}
		b[i] = base62Alphabet[n.Int64()]
	}

	return string(b), nil
}
