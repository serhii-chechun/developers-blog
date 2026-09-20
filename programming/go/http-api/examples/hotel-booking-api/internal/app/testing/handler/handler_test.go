package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"hotel-booking-api/internal/app/testing/handler/mock"
	modelTesting "hotel-booking-api/internal/app/testing/model"
	modelHotel "hotel-booking-api/internal/hotel/model"
	modelRoom "hotel-booking-api/internal/room/model"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

var errDefault = errors.New("error default")

func TestTestingHandler_Seed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Parallel()

	const seedBody = `{"hotels":[{"id":"h1","name":"1 Mercury Serenity","address":"Address 1, Test City","phone":"+44 1234 900001"}],"rooms":[{"id":"r1","room_label":"Room 1","is_available":true,"hotel_id":"h1","room_type":{"room_type_caption":"Double","room_type_capacity":2}}],"room_types":[{"room_type_caption":"Double","room_type_capacity":2}]}`

	result := &modelTesting.SeedResult{
		Hotels: []*modelHotel.HotelItem{
			{ID: "h1", Name: "1 Mercury Serenity", Address: "Address 1, Test City", Phone: "+44 1234 900001"},
		},
		Rooms: []*modelRoom.RoomItem{
			{
				ID:          "r1",
				HotelID:     "h1",
				Label:       "Room 1",
				IsAvailable: true,
				TypeID:      1,
				RoomType:    &modelRoom.RoomType{ID: 1, Caption: "Double", Capacity: 2},
			},
		},
		RoomTypes: []*modelRoom.RoomType{
			{ID: 1, Caption: "Double", Capacity: 2},
		},
	}

	tests := []struct {
		name             string
		body             string
		expect           func(t *testing.T, service *mock.MockTestingService)
		wantStatus       int
		wantBody         string
		wantBodyContains string
	}{
		{
			name: "seeds the requested limits",
			body: `{"max_hotels":2,"max_rooms_per_hotel":4}`,
			expect: func(t *testing.T, service *mock.MockTestingService) {
				service.EXPECT().
					SeedTestData(t.Context(), modelTesting.SeedParams{Hotels: 2, RoomsPerHotel: 4}).
					Return(result, nil).
					Once()
			},
			wantStatus: http.StatusOK,
			wantBody:   seedBody,
		},
		{
			name: "falls back to the default limits",
			body: "{}",
			expect: func(t *testing.T, service *mock.MockTestingService) {
				service.EXPECT().
					SeedTestData(t.Context(), modelTesting.SeedParams{Hotels: 3, RoomsPerHotel: 6}).
					Return(result, nil).
					Once()
			},
			wantStatus: http.StatusOK,
			wantBody:   seedBody,
		},
		{
			name: "maps missing room types to conflict",
			body: "{}",
			expect: func(t *testing.T, service *mock.MockTestingService) {
				service.EXPECT().
					SeedTestData(t.Context(), modelTesting.SeedParams{Hotels: 3, RoomsPerHotel: 6}).
					Return(nil, modelTesting.ErrNoRoomTypes).
					Once()
			},
			wantStatus: http.StatusConflict,
			wantBody:   `{"error":"room types are not seeded"}`,
		},
		{
			name: "maps an unexpected failure to internal server error",
			body: "{}",
			expect: func(t *testing.T, service *mock.MockTestingService) {
				service.EXPECT().
					SeedTestData(t.Context(), modelTesting.SeedParams{Hotels: 3, RoomsPerHotel: 6}).
					Return(nil, errDefault).
					Once()
			},
			wantStatus: http.StatusInternalServerError,
			wantBody:   `{"error":"error default"}`,
		},
		{
			name:       "rejects a hotel count above the maximum",
			body:       `{"max_hotels":101}`,
			wantStatus: http.StatusBadRequest,
			wantBody:   `{"error":"max_hotels must not exceed 100"}`,
		},
		{
			name:       "rejects a rooms per hotel count above the maximum",
			body:       `{"max_rooms_per_hotel":1001}`,
			wantStatus: http.StatusBadRequest,
			wantBody:   `{"error":"max_rooms_per_hotel must not exceed 1000"}`,
		},
		{
			name:             "rejects a negative rooms per hotel",
			body:             `{"max_rooms_per_hotel":-1}`,
			wantStatus:       http.StatusBadRequest,
			wantBodyContains: "MaxRoomsPerHotel",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := mock.NewMockTestingService(t)

			if tt.expect != nil {
				tt.expect(t, service)
			}

			recorder := httptest.NewRecorder()

			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPut, "/v1/testing", strings.NewReader(tt.body)).WithContext(t.Context())
			c.Request.Header.Set("Content-Type", "application/json")

			New(service).Seed(c)

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

func TestTestingHandler_Reset(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Parallel()

	tests := []struct {
		name       string
		expect     func(t *testing.T, service *mock.MockTestingService)
		wantStatus int
		wantBody   string
	}{
		{
			name: "resets the test data",
			expect: func(t *testing.T, service *mock.MockTestingService) {
				service.EXPECT().ResetTestData(t.Context()).Return(nil).Once()
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name: "reports a service failure",
			expect: func(t *testing.T, service *mock.MockTestingService) {
				service.EXPECT().ResetTestData(t.Context()).Return(errDefault).Once()
			},
			wantStatus: http.StatusInternalServerError,
			wantBody:   `{"error":"error default"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := mock.NewMockTestingService(t)

			if tt.expect != nil {
				tt.expect(t, service)
			}

			engine := gin.New()
			engine.DELETE("/v1/testing", New(service).Reset)

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodDelete, "/v1/testing", nil).WithContext(t.Context())

			engine.ServeHTTP(recorder, request)

			assert.Equal(t, tt.wantStatus, recorder.Code)

			if tt.wantBody != "" {
				assert.JSONEq(t, tt.wantBody, recorder.Body.String())
			} else {
				assert.Empty(t, recorder.Body.String())
			}
		})
	}
}
