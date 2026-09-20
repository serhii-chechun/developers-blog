package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"hotel-booking-api/internal/helper/pagination"
	"hotel-booking-api/internal/room/handler/mock"
	"hotel-booking-api/internal/room/model"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errDefault = errors.New("error default")

func TestRoomHandler_GetAll(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Parallel()

	today := time.Now().UTC().Truncate(24 * time.Hour)
	checkIn := today.AddDate(0, 0, 5).Format(time.DateOnly)
	checkOut := today.AddDate(0, 0, 7).Format(time.DateOnly)

	const pageBody = `{"rooms":[{"id":"r1","room_label":"101","is_available":true,"room_type":{"room_type_caption":"Double","room_type_capacity":2},"hotel_id":"h1","hotel_details":{"hotel_name":"Grand Hotel","hotel_address":"1 Main St"}}],"page_size":5}`

	page := &model.RoomsPage{
		Rooms: []*model.RoomItem{
			{
				ID:          "r1",
				HotelID:     "h1",
				Label:       "101",
				IsAvailable: true,
				TypeID:      1,
				RoomType:    &model.RoomType{ID: 1, Caption: "Double", Capacity: 2},
				RoomItemHotelDetails: &model.RoomItemHotelDetails{
					Name:    "Grand Hotel",
					Address: "1 Main St",
				},
			},
		},
		PageSize: 5,
	}

	emptyPage := &model.RoomsPage{Rooms: []*model.RoomItem{}, PageSize: pagination.DefaultPageSize}

	token, err := pagination.Encode("r1")
	require.NoError(t, err)

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

	params := model.FindAvailableRoomsParams{
		CheckInDate:  checkIn,
		CheckOutDate: checkOut,
		GuestsCount:  2,
		HotelID:      "h1",
		PageSize:     5,
	}

	tests := []struct {
		name             string
		query            string
		expect           func(t *testing.T, service *mock.MockRoomService)
		wantStatus       int
		wantBody         string
		wantBodyContains string
	}{
		{
			name:  "returns the page of the service",
			query: query(func(url.Values) {}),
			expect: func(t *testing.T, service *mock.MockRoomService) {
				service.EXPECT().
					FindAllAvailableRooms(t.Context(), params).
					Return(page, nil).
					Once()
			},
			wantStatus: http.StatusOK,
			wantBody:   pageBody,
		},
		{
			name:  "forwards the decoded cursor to the service",
			query: query(func(values url.Values) { values.Set("next_page", token) }),
			expect: func(t *testing.T, service *mock.MockRoomService) {
				withCursor := params
				withCursor.AfterID = "r1"

				service.EXPECT().
					FindAllAvailableRooms(t.Context(), withCursor).
					Return(page, nil).
					Once()
			},
			wantStatus: http.StatusOK,
			wantBody:   pageBody,
		},
		{
			name:  "returns an empty page",
			query: query(func(values url.Values) { values.Del("hotel_id") }),
			expect: func(t *testing.T, service *mock.MockRoomService) {
				withoutHotel := params
				withoutHotel.HotelID = ""

				service.EXPECT().
					FindAllAvailableRooms(t.Context(), withoutHotel).
					Return(emptyPage, nil).
					Once()
			},
			wantStatus: http.StatusOK,
			wantBody:   `{"rooms":[],"page_size":20}`,
		},
		{
			name:  "returns null when the service has no page",
			query: query(func(url.Values) {}),
			expect: func(t *testing.T, service *mock.MockRoomService) {
				service.EXPECT().
					FindAllAvailableRooms(t.Context(), params).
					Return(nil, nil).
					Once()
			},
			wantStatus: http.StatusOK,
			wantBody:   `null`,
		},
		{
			name:  "reports a service failure",
			query: query(func(url.Values) {}),
			expect: func(t *testing.T, service *mock.MockRoomService) {
				service.EXPECT().
					FindAllAvailableRooms(t.Context(), params).
					Return(nil, errDefault).
					Once()
			},
			wantStatus: http.StatusInternalServerError,
			wantBody:   `{"error":"error default"}`,
		},
		{
			name:             "rejects a missing guests count",
			query:            query(func(values url.Values) { values.Del("guests_count") }),
			wantStatus:       http.StatusBadRequest,
			wantBodyContains: "GuestsCount",
		},
		{
			name:       "rejects an unknown next page token",
			query:      query(func(values url.Values) { values.Set("next_page", "not_a_token!!") }),
			wantStatus: http.StatusBadRequest,
			wantBody:   `{"error":"invalid next_page token"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := mock.NewMockRoomService(t)

			if tt.expect != nil {
				tt.expect(t, service)
			}

			recorder := httptest.NewRecorder()

			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodGet, "/v1/rooms?"+tt.query, nil).WithContext(t.Context())

			New(service).GetAll(c)

			assert.Equal(t, tt.wantStatus, recorder.Code)

			if tt.wantBody != "" {
				assert.JSONEq(t, tt.wantBody, recorder.Body.String())
			}

			if tt.wantBodyContains != "" {
				assert.Contains(t, recorder.Body.String(), tt.wantBodyContains)
			}
		})
	}
}
