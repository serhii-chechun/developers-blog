package handler

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"hotel-booking-api/internal/reservation/handler/mock"
	"hotel-booking-api/internal/reservation/model"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

var errDefault = errors.New("error default")

func TestReservationHandler_Get(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Parallel()

	const reservationBody = `{"id":"res-1","check_in_date":"2026-10-01","check_out_date":"2026-10-03","guest_full_name":"Jane Doe","guest_email":"jane@example.com","guests_count":2,"reference":"REF-1","status":2000,"hotel_id":"h1","hotel_details":{"name":"Grand Hotel","address":"1 Main St"},"rooms":[{"room_id":"r1","room_label":"101","room_type_caption":"Double","room_type_capacity":2,"guests_count":2}]}`

	reservation := &model.ReservationItem{
		ID:            "res-1",
		CheckInDate:   "2026-10-01",
		CheckOutDate:  "2026-10-03",
		GuestFullName: "Jane Doe",
		GuestEmail:    "jane@example.com",
		GuestsCount:   2,
		Reference:     "REF-1",
		Status:        model.ReservationStatusPending,
		HotelID:       "h1",
		ReservationHotelDetails: &model.ReservationHotelDetails{
			Name:    "Grand Hotel",
			Address: "1 Main St",
		},
		Rooms: []*model.ReservationRoomDetails{
			{RoomID: "r1", Label: "101", RoomTypeCaption: "Double", RoomTypeCapacity: 2, GuestsCount: 2},
		},
	}

	tests := []struct {
		name         string
		expect       func(t *testing.T, service *mock.MockReservationService)
		wantStatus   int
		wantBody     string
		wantNonEmpty bool
	}{
		{
			name: "returns the reservation of the service",
			expect: func(t *testing.T, service *mock.MockReservationService) {
				service.EXPECT().
					FindReservationByReference(t.Context(), "REF-1").
					Return(reservation, nil).
					Once()
			},
			wantStatus: http.StatusOK,
			wantBody:   reservationBody,
		},
		{
			name: "reports an unknown booking reference",
			expect: func(t *testing.T, service *mock.MockReservationService) {
				service.EXPECT().
					FindReservationByReference(t.Context(), "REF-1").
					Return(nil, nil).
					Once()
			},
			wantStatus: http.StatusNotFound,
			wantBody:   `{"error":"reservation not found"}`,
		},
		{
			name: "reports a service failure",
			expect: func(t *testing.T, service *mock.MockReservationService) {
				service.EXPECT().
					FindReservationByReference(t.Context(), "REF-1").
					Return(nil, errDefault).
					Once()
			},
			wantStatus: http.StatusInternalServerError,
			wantBody:   `{"error":"error default"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := mock.NewMockReservationService(t)

			if tt.expect != nil {
				tt.expect(t, service)
			}

			recorder := httptest.NewRecorder()

			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodGet, "/v1/reservations/REF-1", nil).WithContext(t.Context())
			c.Params = gin.Params{{Key: "booking_ref", Value: "REF-1"}}

			New(service).Get(c)

			assert.Equal(t, tt.wantStatus, recorder.Code)

			if tt.wantBody != "" {
				assert.JSONEq(t, tt.wantBody, recorder.Body.String())
			}
		})
	}
}

func TestReservationHandler_Create(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Parallel()

	today := time.Now().UTC().Truncate(24 * time.Hour)
	checkIn := today.AddDate(0, 0, 5).Format(time.DateOnly)
	checkOut := today.AddDate(0, 0, 7).Format(time.DateOnly)

	const createdBody = `{"id":"res-1","check_in_date":"2026-10-01","check_out_date":"2026-10-03","guest_full_name":"Jane Doe","guest_email":"jane@example.com","guests_count":2,"reference":"REF-1","status":2000,"hotel_id":"h1","hotel_details":{"name":"Grand Hotel","address":"1 Main St"},"rooms":[{"room_id":"r1","room_label":"101","room_type_caption":"Double","room_type_capacity":2,"guests_count":2}]}`

	validBody := fmt.Sprintf(
		`{"hotel_id":"h1","check_in_date":%q,"check_out_date":%q,"guest_full_name":"Jane Doe","guest_email":"jane@example.com","guests_count":2,"room_ids":["r1"]}`,
		checkIn,
		checkOut,
	)

	booking := model.ReservationItem{
		HotelID:       "h1",
		CheckInDate:   checkIn,
		CheckOutDate:  checkOut,
		GuestFullName: "Jane Doe",
		GuestEmail:    "jane@example.com",
		GuestsCount:   2,
		RoomIDs:       []string{"r1"},
	}

	created := &model.ReservationItem{
		ID:            "res-1",
		CheckInDate:   "2026-10-01",
		CheckOutDate:  "2026-10-03",
		GuestFullName: "Jane Doe",
		GuestEmail:    "jane@example.com",
		GuestsCount:   2,
		Reference:     "REF-1",
		Status:        model.ReservationStatusPending,
		HotelID:       "h1",
		ReservationHotelDetails: &model.ReservationHotelDetails{
			Name:    "Grand Hotel",
			Address: "1 Main St",
		},
		Rooms: []*model.ReservationRoomDetails{
			{RoomID: "r1", Label: "101", RoomTypeCaption: "Double", RoomTypeCapacity: 2, GuestsCount: 2},
		},
	}

	tests := []struct {
		name             string
		body             string
		expect           func(t *testing.T, service *mock.MockReservationService)
		wantStatus       int
		wantBody         string
		wantBodyContains string
	}{
		{
			name: "creates the reservation of the service",
			body: validBody,
			expect: func(t *testing.T, service *mock.MockReservationService) {
				service.EXPECT().
					CreateReservation(t.Context(), booking).
					Return(created, nil).
					Once()
			},
			wantStatus: http.StatusOK,
			wantBody:   createdBody,
		},
		{
			name: "maps an unknown hotel to not found",
			body: validBody,
			expect: func(t *testing.T, service *mock.MockReservationService) {
				service.EXPECT().
					CreateReservation(t.Context(), booking).
					Return(nil, model.ErrHotelNotFound).
					Once()
			},
			wantStatus: http.StatusNotFound,
			wantBody:   `{"error":"hotel not found"}`,
		},
		{
			name: "maps an unknown room to not found",
			body: validBody,
			expect: func(t *testing.T, service *mock.MockReservationService) {
				service.EXPECT().
					CreateReservation(t.Context(), booking).
					Return(nil, model.ErrRoomNotFound).
					Once()
			},
			wantStatus: http.StatusNotFound,
			wantBody:   `{"error":"room not found"}`,
		},
		{
			name: "maps an unavailable room to conflict",
			body: validBody,
			expect: func(t *testing.T, service *mock.MockReservationService) {
				service.EXPECT().
					CreateReservation(t.Context(), booking).
					Return(nil, model.ErrRoomUnavailable).
					Once()
			},
			wantStatus: http.StatusConflict,
			wantBody:   `{"error":"room is not available for the requested dates"}`,
		},
		{
			name: "maps an insufficient capacity to conflict",
			body: validBody,
			expect: func(t *testing.T, service *mock.MockReservationService) {
				service.EXPECT().
					CreateReservation(t.Context(), booking).
					Return(nil, model.ErrInsufficientCapacity).
					Once()
			},
			wantStatus: http.StatusConflict,
			wantBody:   `{"error":"no combination of available rooms can accommodate the requested guests"}`,
		},
		{
			name: "maps a taken reference to conflict",
			body: validBody,
			expect: func(t *testing.T, service *mock.MockReservationService) {
				service.EXPECT().
					CreateReservation(t.Context(), booking).
					Return(nil, model.ErrReferenceExists).
					Once()
			},
			wantStatus: http.StatusConflict,
			wantBody:   `{"error":"booking reference already exists"}`,
		},
		{
			name: "maps an unexpected failure to internal server error",
			body: validBody,
			expect: func(t *testing.T, service *mock.MockReservationService) {
				service.EXPECT().
					CreateReservation(t.Context(), booking).
					Return(nil, errDefault).
					Once()
			},
			wantStatus: http.StatusInternalServerError,
			wantBody:   `{"error":"error default"}`,
		},
		{
			name:             "rejects a request without a hotel",
			body:             fmt.Sprintf(`{"check_in_date":%q,"check_out_date":%q,"guest_full_name":"Jane Doe","guest_email":"jane@example.com","guests_count":2}`, checkIn, checkOut),
			wantStatus:       http.StatusBadRequest,
			wantBodyContains: "HotelID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := mock.NewMockReservationService(t)

			if tt.expect != nil {
				tt.expect(t, service)
			}

			recorder := httptest.NewRecorder()

			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/reservations", strings.NewReader(tt.body)).WithContext(t.Context())
			c.Request.Header.Set("Content-Type", "application/json")

			New(service).Create(c)

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
