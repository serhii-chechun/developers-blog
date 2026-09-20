package pagination

import (
	"encoding/base64"
	"encoding/json"
	"errors"
)

const (
	// DefaultPageSize is used when the client does not request a specific page size.
	DefaultPageSize = 20

	// MaxPageSize is the upper bound for the number of items returned per page.
	MaxPageSize = 100
)

// ErrInvalidToken is returned when an opaque page token cannot be decoded.
var ErrInvalidToken = errors.New("invalid next_page token")

type (
	// pageToken is the decoded representation of an opaque pagination token.
	pageToken struct {
		AfterID string `json:"after_id"`
	}
)

// NormalizeSize clamps the requested page size into the allowed range.
func NormalizeSize(size int) int {
	switch {
	case size <= 0:
		return DefaultPageSize
	case size > MaxPageSize:
		return MaxPageSize
	default:
		return size
	}
}

// Encode encodes the cursor into an opaque token.
func Encode(afterID string) (string, error) {
	raw, err := json.Marshal(pageToken{AfterID: afterID})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// Decode decodes an opaque token into its cursor.
func Decode(token string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return "", ErrInvalidToken
	}

	var t pageToken
	if err := json.Unmarshal(raw, &t); err != nil || t.AfterID == "" {
		return "", ErrInvalidToken
	}

	return t.AfterID, nil
}
