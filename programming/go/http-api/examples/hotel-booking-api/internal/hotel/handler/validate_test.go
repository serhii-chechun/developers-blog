package handler

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"hotel-booking-api/internal/helper/pagination"
	"hotel-booking-api/internal/hotel/model"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateGetAllRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Parallel()

	token, err := pagination.Encode("h1")
	require.NoError(t, err)

	notACursor := base64.RawURLEncoding.EncodeToString([]byte("nope"))

	tests := []struct {
		name       string
		query      string
		want       model.FindHotelsParams
		wantErr    error
		wantAnyErr bool
	}{
		{
			name:  "binds the pattern and keeps the requested page size",
			query: "name_pattern=grand&page_size=5",
			want:  model.FindHotelsParams{NamePattern: "grand", PageSize: 5},
		},
		{
			name:  "falls back to the default page size",
			query: "",
			want:  model.FindHotelsParams{PageSize: pagination.DefaultPageSize},
		},
		{
			name:  "clamps a page size above the maximum",
			query: "page_size=500",
			want:  model.FindHotelsParams{PageSize: pagination.MaxPageSize},
		},
		{
			name:  "clamps a negative page size to the default",
			query: "page_size=-3",
			want:  model.FindHotelsParams{PageSize: pagination.DefaultPageSize},
		},
		{
			name:  "decodes the next page token into the cursor",
			query: "name_pattern=grand&next_page=" + token,
			want: model.FindHotelsParams{
				NamePattern: "grand",
				PageSize:    pagination.DefaultPageSize,
				AfterID:     "h1",
			},
		},
		{
			name:  "keeps the pattern when the next page token is empty",
			query: "name_pattern=grand&next_page=",
			want:  model.FindHotelsParams{NamePattern: "grand", PageSize: pagination.DefaultPageSize},
		},
		{
			name:  "ignores unknown query parameters",
			query: "foo=bar&name_pattern=grand",
			want:  model.FindHotelsParams{NamePattern: "grand", PageSize: pagination.DefaultPageSize},
		},
		{
			name:    "rejects a token that is not base64",
			query:   "name_pattern=grand&page_size=5&next_page=not_a_token!!",
			want:    model.FindHotelsParams{NamePattern: "grand", PageSize: 5},
			wantErr: pagination.ErrInvalidToken,
		},
		{
			name:    "rejects a token that is not a cursor",
			query:   "page_size=5&next_page=" + notACursor,
			want:    model.FindHotelsParams{PageSize: 5},
			wantErr: pagination.ErrInvalidToken,
		},
		{
			name:       "rejects an unparsable page size",
			query:      "name_pattern=grand&page_size=abc",
			want:       model.FindHotelsParams{},
			wantAnyErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/hotels?"+tt.query, nil)

			got, err := validateGetAllRequest(c)

			switch {
			case tt.wantErr != nil:
				require.ErrorIs(t, err, tt.wantErr)
			case tt.wantAnyErr:
				require.Error(t, err)
			default:
				require.NoError(t, err)
			}

			assert.Equal(t, tt.want, got)
		})
	}
}
