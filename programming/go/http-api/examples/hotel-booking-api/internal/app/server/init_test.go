package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	appMock "hotel-booking-api/internal/app/server/mock"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type handlerMocks struct {
	hotels       *appMock.MockHotelHandler
	rooms        *appMock.MockRoomHandler
	reservations *appMock.MockReservationHandler
	testing      *appMock.MockTestingHandler
}

func newHandlerMocks(t *testing.T) handlerMocks {
	t.Helper()

	return handlerMocks{
		hotels:       appMock.NewMockHotelHandler(t),
		rooms:        appMock.NewMockRoomHandler(t),
		reservations: appMock.NewMockReservationHandler(t),
		testing:      appMock.NewMockTestingHandler(t),
	}
}

func TestRegisterRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Parallel()

	tests := []struct {
		name       string
		method     string
		target     string
		wantStatus int
		wantBody   string
		expect     func(m handlerMocks)
	}{
		{
			name:       "answers the health check",
			method:     http.MethodGet,
			target:     "/health",
			wantStatus: http.StatusOK,
			wantBody:   `{"status":"ok"}`,
		},
		{
			name:       "lists the hotels",
			method:     http.MethodGet,
			target:     "/v1/hotels",
			wantStatus: http.StatusOK,
			expect: func(m handlerMocks) {
				m.hotels.EXPECT().GetAll(mock.Anything).Once()
			},
		},
		{
			name:       "lists the rooms",
			method:     http.MethodGet,
			target:     "/v1/rooms",
			wantStatus: http.StatusOK,
			expect: func(m handlerMocks) {
				m.rooms.EXPECT().GetAll(mock.Anything).Once()
			},
		},
		{
			name:       "creates a reservation",
			method:     http.MethodPost,
			target:     "/v1/reservations",
			wantStatus: http.StatusOK,
			expect: func(m handlerMocks) {
				m.reservations.EXPECT().Create(mock.Anything).Once()
			},
		},
		{
			name:       "reads a reservation by its booking reference",
			method:     http.MethodGet,
			target:     "/v1/reservations/REF-1",
			wantStatus: http.StatusOK,
			expect: func(m handlerMocks) {
				m.reservations.EXPECT().
					Get(mock.MatchedBy(func(c *gin.Context) bool {
						return c.Param("booking_ref") == "REF-1"
					})).
					Once()
			},
		},
		{
			name:       "seeds the test data",
			method:     http.MethodPut,
			target:     "/v1/testing",
			wantStatus: http.StatusOK,
			expect: func(m handlerMocks) {
				m.testing.EXPECT().Seed(mock.Anything).Once()
			},
		},
		{
			name:       "resets the test data",
			method:     http.MethodDelete,
			target:     "/v1/testing",
			wantStatus: http.StatusOK,
			expect: func(m handlerMocks) {
				m.testing.EXPECT().Reset(mock.Anything).Once()
			},
		},
		{
			name:       "rejects an unknown path",
			method:     http.MethodGet,
			target:     "/v1/unknown",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "rejects an unregistered method",
			method:     http.MethodDelete,
			target:     "/v1/hotels",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "redirects a trailing slash",
			method:     http.MethodGet,
			target:     "/v1/hotels/",
			wantStatus: http.StatusMovedPermanently,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mocks := newHandlerMocks(t)

			engine := gin.New()

			routes := (&apiServer{}).registerRoutes(engine, &handlers{
				hotelHandler:       mocks.hotels,
				roomHandler:        mocks.rooms,
				reservationHandler: mocks.reservations,
				testingHandler:     mocks.testing,
			})

			require.Same(t, engine, routes)

			if tt.expect != nil {
				tt.expect(mocks)
			}

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.target, nil)

			routes.ServeHTTP(recorder, request)

			assert.Equal(t, tt.wantStatus, recorder.Code)

			if tt.wantBody != "" {
				assert.JSONEq(t, tt.wantBody, recorder.Body.String())
			}
		})
	}
}
