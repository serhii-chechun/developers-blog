package handler

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"hotel-booking-api/internal/reservation/model"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateCreateRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Parallel()

	today := time.Now().UTC().Truncate(24 * time.Hour)

	checkInDay := today.AddDate(0, 0, 5)
	checkOutDay := today.AddDate(0, 0, 7)

	checkIn := checkInDay.Format(time.DateOnly)
	checkOut := checkOutDay.Format(time.DateOnly)

	validItem := model.ReservationItem{
		HotelID:       "h1",
		CheckInDate:   checkIn,
		CheckOutDate:  checkOut,
		GuestFullName: "Jane Doe",
		GuestEmail:    "jane@example.com",
		GuestsCount:   2,
	}

	validWithRooms := validItem
	validWithRooms.RoomIDs = []string{"r1", "r2"}

	baseQuery := url.Values{
		"hotel_id":        {"h1"},
		"check_in_date":   {checkIn},
		"check_out_date":  {checkOut},
		"guest_full_name": {"Jane Doe"},
		"guest_email":     {"jane@example.com"},
		"guests_count":    {"2"},
	}

	query := func(mutate func(values url.Values)) string {
		values := url.Values{}
		for key, value := range baseQuery {
			values[key] = value
		}
		mutate(values)

		return values.Encode()
	}

	jsonBody := fmt.Sprintf(
		`{"hotel_id":"h1","check_in_date":%q,"check_out_date":%q,"guest_full_name":"Jane Doe","guest_email":"jane@example.com","guests_count":2,"room_ids":["r1","r2"]}`,
		checkIn,
		checkOut,
	)

	tests := []struct {
		name        string
		contentType string
		body        string
		query       string
		want        model.ReservationItem
		wantErrText string
		wantAnyErr  bool
	}{
		{
			name:  "binds the query string into the reservation",
			query: query(func(url.Values) {}),
			want:  validItem,
		},
		{
			name:  "binds repeated room ids from the query string",
			query: query(func(values url.Values) { values["room_ids"] = []string{"r1", "r2"} }),
			want:  validWithRooms,
		},
		{
			name:        "binds the json body into the reservation",
			contentType: "application/json",
			body:        jsonBody,
			want:        validWithRooms,
		},
		{
			name: "accepts today as the check in date",
			query: query(func(values url.Values) {
				values.Set("check_in_date", today.Format(time.DateOnly))
				values.Set("check_out_date", today.AddDate(0, 0, 1).Format(time.DateOnly))
			}),
			want: model.ReservationItem{
				HotelID:       "h1",
				CheckInDate:   today.Format(time.DateOnly),
				CheckOutDate:  today.AddDate(0, 0, 1).Format(time.DateOnly),
				GuestFullName: "Jane Doe",
				GuestEmail:    "jane@example.com",
				GuestsCount:   2,
			},
		},
		{
			name:       "rejects a missing hotel id",
			query:      query(func(values url.Values) { values.Del("hotel_id") }),
			wantAnyErr: true,
		},
		{
			name:       "rejects a non positive guests count",
			query:      query(func(values url.Values) { values.Set("guests_count", "0") }),
			wantAnyErr: true,
		},
		{
			name:       "rejects a malformed guest email",
			query:      query(func(values url.Values) { values.Set("guest_email", "not-an-email") }),
			wantAnyErr: true,
		},
		{
			name:        "rejects a malformed json body",
			contentType: "application/json",
			body:        `{"hotel_id":"h1",`,
			wantAnyErr:  true,
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}

			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/reservations?"+tt.query, body)
			if tt.contentType != "" {
				c.Request.Header.Set("Content-Type", tt.contentType)
			}

			got, err := validateCreateRequest(c)

			switch {
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
