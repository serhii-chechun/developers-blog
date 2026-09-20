package service

import (
	"context"
	"errors"
	"testing"

	"hotel-booking-api/internal/hotel/model"
	"hotel-booking-api/internal/hotel/service/mock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errDefault = errors.New("error default")

func newTestService(t *testing.T, expectations func(repo *mock.MockHotelRepository)) *hotelService {
	t.Helper()

	repo := mock.NewMockHotelRepository(t)
	expectations(repo)

	return New(repo)
}

func TestHotelService_FindAllHotels(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	page := &model.HotelsPage{
		Hotels: []*model.HotelItem{
			{ID: "h1", Name: "Grand Hotel", Address: "1 Main St", Phone: "111"},
		},
		PageSize: 20,
		NextPage: "eyJhZnRlcl9pZCI6ImgxIn0",
	}

	emptyPage := &model.HotelsPage{Hotels: []*model.HotelItem{}, PageSize: 5}

	tests := []struct {
		name         string
		params       model.FindHotelsParams
		expectations func(repo *mock.MockHotelRepository)
		want         *model.HotelsPage
		wantErr      error
	}{
		{
			name:   "forwards the params and returns the page of the repository",
			params: model.FindHotelsParams{NamePattern: "grand", AfterID: "h0", PageSize: 20},
			expectations: func(repo *mock.MockHotelRepository) {
				repo.EXPECT().
					GetHotelsByName(ctx, model.FindHotelsParams{NamePattern: "grand", AfterID: "h0", PageSize: 20}).
					Return(page, nil).
					Once()
			},
			want: page,
		},
		{
			name:   "returns an empty page as is",
			params: model.FindHotelsParams{NamePattern: "nowhere", PageSize: 5},
			expectations: func(repo *mock.MockHotelRepository) {
				repo.EXPECT().
					GetHotelsByName(ctx, model.FindHotelsParams{NamePattern: "nowhere", PageSize: 5}).
					Return(emptyPage, nil).
					Once()
			},
			want: emptyPage,
		},
		{
			name:   "propagates the repository error",
			params: model.FindHotelsParams{NamePattern: "grand", PageSize: 20},
			expectations: func(repo *mock.MockHotelRepository) {
				repo.EXPECT().
					GetHotelsByName(ctx, model.FindHotelsParams{NamePattern: "grand", PageSize: 20}).
					Return(nil, errDefault).
					Once()
			},
			wantErr: errDefault,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := newTestService(t, tt.expectations).FindAllHotels(ctx, tt.params)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, got)
				return
			}

			require.NoError(t, err)
			assert.Same(t, tt.want, got)
		})
	}
}

func TestHotelService_CreateHotels(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	hotels := []*model.HotelItem{
		{ID: "h1", Name: "Grand Hotel", Address: "1 Main St", Phone: "111"},
		{ID: "h2", Name: "Grand Plaza", Address: "2 Main St", Phone: "222"},
	}

	tests := []struct {
		name         string
		hotels       []*model.HotelItem
		expectations func(repo *mock.MockHotelRepository)
		wantErr      error
	}{
		{
			name:   "stores the whole batch through the repository",
			hotels: hotels,
			expectations: func(repo *mock.MockHotelRepository) {
				repo.EXPECT().PutHotels(ctx, hotels).Return(nil).Once()
			},
		},
		{
			name:   "passes an empty batch through unchanged",
			hotels: []*model.HotelItem{},
			expectations: func(repo *mock.MockHotelRepository) {
				repo.EXPECT().PutHotels(ctx, []*model.HotelItem{}).Return(nil).Once()
			},
		},
		{
			name:   "propagates the repository error",
			hotels: hotels,
			expectations: func(repo *mock.MockHotelRepository) {
				repo.EXPECT().PutHotels(ctx, hotels).Return(errDefault).Once()
			},
			wantErr: errDefault,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := newTestService(t, tt.expectations).CreateHotels(ctx, tt.hotels)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestHotelService_RemoveAllHotels(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	tests := []struct {
		name         string
		expectations func(repo *mock.MockHotelRepository)
		wantErr      error
	}{
		{
			name: "removes everything through the repository",
			expectations: func(repo *mock.MockHotelRepository) {
				repo.EXPECT().DeleteAllHotels(ctx).Return(nil).Once()
			},
		},
		{
			name: "propagates the repository error",
			expectations: func(repo *mock.MockHotelRepository) {
				repo.EXPECT().DeleteAllHotels(ctx).Return(errDefault).Once()
			},
			wantErr: errDefault,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := newTestService(t, tt.expectations).RemoveAllHotels(ctx)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
