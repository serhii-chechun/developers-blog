package handler

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"hotel-booking-api/internal/helper/pagination"
	"hotel-booking-api/internal/room/model"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateGetAllRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Parallel()

	today := time.Now().UTC().Truncate(24 * time.Hour)

	checkIn := today.AddDate(0, 0, 5).Format(time.DateOnly)
	checkOut := today.AddDate(0, 0, 7).Format(time.DateOnly)
	tomorrow := today.AddDate(0, 0, 1).Format(time.DateOnly)

	token, err := pagination.Encode("r1")
	require.NoError(t, err)

	notACursor := base64.RawURLEncoding.EncodeToString([]byte("nope"))

	baseQuery := url.Values{
		"guests_count":   {"2"},
		"check_in_date":  {checkIn},
		"check_out_date": {checkOut},
		"hotel_id":       {"h1"},
		"page_size":      {"5"},
	}

	query := func(mutate func(values url.Values)) string {
		values := url.Values{}
		for key, value := range baseQuery {
			values[key] = value
		}
		mutate(values)

		return values.Encode()
	}

	validParams := model.FindAvailableRoomsParams{
		CheckInDate:  checkIn,
		CheckOutDate: checkOut,
		GuestsCount:  2,
		HotelID:      "h1",
		PageSize:     5,
	}

	paramsWith := func(mutate func(params *model.FindAvailableRoomsParams)) model.FindAvailableRoomsParams {
		params := validParams
		mutate(&params)

		return params
	}

	tests := []struct {
		name        string
		query       string
		want        model.FindAvailableRoomsParams
		wantErrIs   error
		wantErrText string
		wantAnyErr  bool
	}{
		{
			name:  "binds the query string into the search params",
			query: query(func(url.Values) {}),
			want:  validParams,
		},
		{
			name:  "leaves the hotel filter empty when it is not provided",
			query: query(func(values url.Values) { values.Del("hotel_id") }),
			want:  paramsWith(func(params *model.FindAvailableRoomsParams) { params.HotelID = "" }),
		},
		{
			name: "accepts today as the check in date",
			query: query(func(values url.Values) {
				values.Set("check_in_date", today.Format(time.DateOnly))
				values.Set("check_out_date", tomorrow)
			}),
			want: paramsWith(func(params *model.FindAvailableRoomsParams) {
				params.CheckInDate = today.Format(time.DateOnly)
				params.CheckOutDate = tomorrow
			}),
		},
		{
			name:  "decodes the next page token into the cursor",
			query: query(func(values url.Values) { values.Set("next_page", token) }),
			want:  paramsWith(func(params *model.FindAvailableRoomsParams) { params.AfterID = "r1" }),
		},
		{
			name:  "keeps the search params when the next page token is empty",
			query: query(func(values url.Values) { values.Set("next_page", "") }),
			want:  validParams,
		},
		{
			name:  "falls back to the default page size",
			query: query(func(values url.Values) { values.Del("page_size") }),
			want:  paramsWith(func(params *model.FindAvailableRoomsParams) { params.PageSize = pagination.DefaultPageSize }),
		},
		{
			name:  "clamps a page size above the maximum",
			query: query(func(values url.Values) { values.Set("page_size", "500") }),
			want:  paramsWith(func(params *model.FindAvailableRoomsParams) { params.PageSize = pagination.MaxPageSize }),
		},
		{
			name:  "clamps a negative page size to the default",
			query: query(func(values url.Values) { values.Set("page_size", "-1") }),
			want:  paramsWith(func(params *model.FindAvailableRoomsParams) { params.PageSize = pagination.DefaultPageSize }),
		},
		{
			name:       "rejects a missing guests count",
			query:      query(func(values url.Values) { values.Del("guests_count") }),
			wantAnyErr: true,
		},
		{
			name:       "rejects a zero guests count",
			query:      query(func(values url.Values) { values.Set("guests_count", "0") }),
			wantAnyErr: true,
		},
		{
			name:       "rejects an unparsable guests count",
			query:      query(func(values url.Values) { values.Set("guests_count", "two") }),
			wantAnyErr: true,
		},
		{
			name:       "rejects a missing check in date",
			query:      query(func(values url.Values) { values.Del("check_in_date") }),
			wantAnyErr: true,
		},
		{
			name:        "rejects a malformed check in date",
			query:       query(func(values url.Values) { values.Set("check_in_date", "25/09/2026") }),
			wantErrText: "check_in_date must be in YYYY-MM-DD format",
		},
		{
			name:        "rejects a malformed check out date",
			query:       query(func(values url.Values) { values.Set("check_out_date", "soon") }),
			wantErrText: "check_out_date must be in YYYY-MM-DD format",
		},
		{
			name:        "rejects a check in date in the past",
			query:       query(func(values url.Values) { values.Set("check_in_date", today.AddDate(0, 0, -1).Format(time.DateOnly)) }),
			wantErrText: "check_in_date must not be in the past",
		},
		{
			name:        "rejects a check out date on the check in date",
			query:       query(func(values url.Values) { values.Set("check_out_date", checkIn) }),
			wantErrText: "check_out_date must be after check_in_date",
		},
		{
			name:        "rejects a check out date before the check in date",
			query:       query(func(values url.Values) { values.Set("check_out_date", today.Format(time.DateOnly)) }),
			wantErrText: "check_out_date must be after check_in_date",
		},
		{
			name:      "rejects a token that is not base64",
			query:     query(func(values url.Values) { values.Set("next_page", "not_a_token!!") }),
			want:      validParams,
			wantErrIs: pagination.ErrInvalidToken,
		},
		{
			name:      "rejects a token that is not a cursor",
			query:     query(func(values url.Values) { values.Set("next_page", notACursor) }),
			want:      validParams,
			wantErrIs: pagination.ErrInvalidToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/rooms?"+tt.query, nil)

			got, err := validateGetAllRequest(c)

			switch {
			case tt.wantErrIs != nil:
				require.ErrorIs(t, err, tt.wantErrIs)
			case tt.wantErrText != "":
				require.ErrorContains(t, err, tt.wantErrText)
			case tt.wantAnyErr:
				require.Error(t, err)
			default:
				require.NoError(t, err)
			}

			assert.Equal(t, tt.want, got)
		})
	}
}
