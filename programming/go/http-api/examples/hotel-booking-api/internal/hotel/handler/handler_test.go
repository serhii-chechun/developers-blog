package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"hotel-booking-api/internal/helper/pagination"
	"hotel-booking-api/internal/hotel/handler/mock"
	"hotel-booking-api/internal/hotel/model"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errDefault = errors.New("error default")

func TestHotelHandler_GetAll(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Parallel()

	const pageBody = `{"hotels":[{"id":"h1","name":"Grand Hotel","address":"1 Main St","phone":"111"}],"page_size":5}`

	page := &model.HotelsPage{
		Hotels: []*model.HotelItem{
			{ID: "h1", Name: "Grand Hotel", Address: "1 Main St", Phone: "111"},
		},
		PageSize: 5,
	}

	emptyPage := &model.HotelsPage{Hotels: []*model.HotelItem{}, PageSize: pagination.DefaultPageSize}

	token, err := pagination.Encode("h1")
	require.NoError(t, err)

	tests := []struct {
		name             string
		query            string
		expect           func(t *testing.T, service *mock.MockHotelService)
		wantStatus       int
		wantBody         string
		wantBodyContains string
	}{
		{
			name:  "returns the page of the service",
			query: "name_pattern=grand&page_size=5",
			expect: func(t *testing.T, service *mock.MockHotelService) {
				service.EXPECT().
					FindAllHotels(t.Context(), model.FindHotelsParams{NamePattern: "grand", PageSize: 5}).
					Return(page, nil).
					Once()
			},
			wantStatus: http.StatusOK,
			wantBody:   pageBody,
		},
		{
			name:  "forwards the decoded cursor to the service",
			query: "name_pattern=grand&next_page=" + token,
			expect: func(t *testing.T, service *mock.MockHotelService) {
				service.EXPECT().
					FindAllHotels(t.Context(), model.FindHotelsParams{
						NamePattern: "grand",
						PageSize:    pagination.DefaultPageSize,
						AfterID:     "h1",
					}).
					Return(page, nil).
					Once()
			},
			wantStatus: http.StatusOK,
			wantBody:   pageBody,
		},
		{
			name:  "falls back to the default page size",
			query: "",
			expect: func(t *testing.T, service *mock.MockHotelService) {
				service.EXPECT().
					FindAllHotels(t.Context(), model.FindHotelsParams{PageSize: pagination.DefaultPageSize}).
					Return(emptyPage, nil).
					Once()
			},
			wantStatus: http.StatusOK,
			wantBody:   `{"hotels":[],"page_size":20}`,
		},
		{
			name:  "returns null when the service has no page",
			query: "name_pattern=nowhere",
			expect: func(t *testing.T, service *mock.MockHotelService) {
				service.EXPECT().
					FindAllHotels(t.Context(), model.FindHotelsParams{
						NamePattern: "nowhere",
						PageSize:    pagination.DefaultPageSize,
					}).
					Return(nil, nil).
					Once()
			},
			wantStatus: http.StatusOK,
			wantBody:   `null`,
		},
		{
			name:  "reports a service failure",
			query: "name_pattern=grand",
			expect: func(t *testing.T, service *mock.MockHotelService) {
				service.EXPECT().
					FindAllHotels(t.Context(), model.FindHotelsParams{
						NamePattern: "grand",
						PageSize:    pagination.DefaultPageSize,
					}).
					Return(nil, errDefault).
					Once()
			},
			wantStatus: http.StatusInternalServerError,
			wantBody:   `{"error":"error default"}`,
		},
		{
			name:             "rejects an unparsable page size",
			query:            "page_size=abc",
			wantStatus:       http.StatusBadRequest,
			wantBodyContains: "invalid syntax",
		},
		{
			name:       "rejects an unknown next page token",
			query:      "next_page=not_a_token!!",
			wantStatus: http.StatusBadRequest,
			wantBody:   `{"error":"invalid next_page token"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := mock.NewMockHotelService(t)

			if tt.expect != nil {
				tt.expect(t, service)
			}

			recorder := httptest.NewRecorder()

			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodGet, "/v1/hotels?"+tt.query, nil).WithContext(t.Context())

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
