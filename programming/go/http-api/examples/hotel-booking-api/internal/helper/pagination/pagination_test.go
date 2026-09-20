package pagination

import (
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		size int
		want int
	}{
		{name: "keeps a size inside the range", size: 1, want: 1},
		{name: "keeps the default size", size: DefaultPageSize, want: DefaultPageSize},
		{name: "keeps the maximum size", size: MaxPageSize, want: MaxPageSize},
		{name: "replaces zero with the default", size: 0, want: DefaultPageSize},
		{name: "replaces a negative size with the default", size: -1, want: DefaultPageSize},
		{name: "replaces the smallest int with the default", size: math.MinInt, want: DefaultPageSize},
		{name: "clamps a size above the maximum", size: MaxPageSize + 1, want: MaxPageSize},
		{name: "clamps the largest int to the maximum", size: math.MaxInt, want: MaxPageSize},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, NormalizeSize(tt.size))
		})
	}
}

func TestEncode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		afterID string
		want    string
	}{
		{name: "encodes a cursor", afterID: "h1", want: "eyJhZnRlcl9pZCI6ImgxIn0"},
		{name: "encodes another cursor", afterID: "r1", want: "eyJhZnRlcl9pZCI6InIxIn0"},
		{name: "encodes an empty cursor", afterID: "", want: "eyJhZnRlcl9pZCI6IiJ9"},
		{name: "escapes a quote in the cursor", afterID: `a"b`, want: "eyJhZnRlcl9pZCI6ImFcImIifQ"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := Encode(tt.afterID)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDecode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		token   string
		want    string
		wantErr error
	}{
		{name: "decodes a token", token: "eyJhZnRlcl9pZCI6ImgxIn0", want: "h1"},
		{name: "decodes another token", token: "eyJhZnRlcl9pZCI6InIxIn0", want: "r1"},
		{name: "decodes an escaped cursor", token: "eyJhZnRlcl9pZCI6ImFcImIifQ", want: `a"b`},
		{name: "rejects an empty token", token: "", wantErr: ErrInvalidToken},
		{name: "rejects a token that is not base64", token: "not_a_token!!", wantErr: ErrInvalidToken},
		{name: "rejects a token of an invalid length", token: "a", wantErr: ErrInvalidToken},
		{name: "rejects a padded token", token: "eyJhZnRlcl9pZCI6ImgxIn0=", wantErr: ErrInvalidToken},
		{name: "rejects a token that is not json", token: "bm9wZQ", wantErr: ErrInvalidToken},
		{name: "rejects a token without the cursor field", token: "eyJhZnRlcklkIjoiaDEifQ", wantErr: ErrInvalidToken},
		{name: "rejects a token with an empty cursor", token: "eyJhZnRlcl9pZCI6IiJ9", wantErr: ErrInvalidToken},
		{name: "rejects a token with a cursor of the wrong type", token: "eyJhZnRlcl9pZCI6MX0", wantErr: ErrInvalidToken},
		{name: "rejects a token that is not an object", token: "WyJoMSJd", wantErr: ErrInvalidToken},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := Decode(tt.token)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Empty(t, got)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		cursor string
	}{
		{name: "short cursor", cursor: "h1"},
		{name: "uuid cursor", cursor: "0f9a1c4e-7b2d-4f6a-9c3e-5d8b1a2e4f60"},
		{name: "cursor with a space", cursor: "after id"},
		{name: "cursor with a quote", cursor: `a"b`},
		{name: "non ascii cursor", cursor: "hôtel-1"},
		{name: "long cursor", cursor: strings.Repeat("a", 256)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			token, err := Encode(tt.cursor)
			require.NoError(t, err)
			assert.NotContains(t, token, "=")

			got, err := Decode(token)

			require.NoError(t, err)
			assert.Equal(t, tt.cursor, got)
		})
	}
}
