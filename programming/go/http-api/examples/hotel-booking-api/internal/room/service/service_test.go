package service

import (
	"context"
	"errors"
	"testing"

	"hotel-booking-api/internal/room/model"
	"hotel-booking-api/internal/room/service/mock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errDefault = errors.New("error default")

func newTestService(t *testing.T, expectations func(repo *mock.MockRoomRepository)) *roomService {
	t.Helper()

	repo := mock.NewMockRoomRepository(t)
	expectations(repo)

	return New(repo)
}

func TestRoomService_FindAllAvailableRooms(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	page := &model.RoomsPage{
		Rooms: []*model.RoomItem{
			{
				ID:          "r1",
				HotelID:     "h1",
				Label:       "101",
				TypeID:      1,
				RoomType:    &model.RoomType{ID: 1, Caption: "Double", Capacity: 2},
				IsAvailable: true,
				RoomItemHotelDetails: &model.RoomItemHotelDetails{
					Name:    "Grand Hotel",
					Address: "1 Main St",
				},
			},
		},
		PageSize: 20,
		NextPage: "eyJhZnRlcl9pZCI6InIxIn0",
	}

	emptyPage := &model.RoomsPage{Rooms: []*model.RoomItem{}, PageSize: 5}

	tests := []struct {
		name         string
		params       model.FindAvailableRoomsParams
		expectations func(repo *mock.MockRoomRepository)
		want         *model.RoomsPage
		wantErr      error
	}{
		{
			name:   "forwards the params and returns the page of the repository",
			params: model.FindAvailableRoomsParams{CheckInDate: "2026-10-01", CheckOutDate: "2026-10-03", GuestsCount: 2, HotelID: "h1", PageSize: 20},
			expectations: func(repo *mock.MockRoomRepository) {
				repo.EXPECT().
					GetAvailableRooms(ctx, model.FindAvailableRoomsParams{CheckInDate: "2026-10-01", CheckOutDate: "2026-10-03", GuestsCount: 2, HotelID: "h1", PageSize: 20}).
					Return(page, nil).
					Once()
			},
			want: page,
		},
		{
			name:   "returns an empty page as is",
			params: model.FindAvailableRoomsParams{CheckInDate: "2026-10-01", CheckOutDate: "2026-10-03", GuestsCount: 9, PageSize: 5},
			expectations: func(repo *mock.MockRoomRepository) {
				repo.EXPECT().
					GetAvailableRooms(ctx, model.FindAvailableRoomsParams{CheckInDate: "2026-10-01", CheckOutDate: "2026-10-03", GuestsCount: 9, PageSize: 5}).
					Return(emptyPage, nil).
					Once()
			},
			want: emptyPage,
		},
		{
			name:   "propagates the repository error",
			params: model.FindAvailableRoomsParams{CheckInDate: "2026-10-01", CheckOutDate: "2026-10-03", GuestsCount: 2, PageSize: 20},
			expectations: func(repo *mock.MockRoomRepository) {
				repo.EXPECT().
					GetAvailableRooms(ctx, model.FindAvailableRoomsParams{CheckInDate: "2026-10-01", CheckOutDate: "2026-10-03", GuestsCount: 2, PageSize: 20}).
					Return(nil, errDefault).
					Once()
			},
			wantErr: errDefault,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := newTestService(t, tt.expectations).FindAllAvailableRooms(ctx, tt.params)

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

func TestRoomService_FindAllRoomTypes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	roomTypes := []*model.RoomType{
		{ID: 1, Caption: "Double", Capacity: 2},
		{ID: 2, Caption: "Suite", Capacity: 4},
	}

	tests := []struct {
		name         string
		expectations func(repo *mock.MockRoomRepository)
		want         []*model.RoomType
		wantErr      error
	}{
		{
			name: "returns the room types of the repository",
			expectations: func(repo *mock.MockRoomRepository) {
				repo.EXPECT().GetRoomTypes(ctx).Return(roomTypes, nil).Once()
			},
			want: roomTypes,
		},
		{
			name: "returns an empty list as is",
			expectations: func(repo *mock.MockRoomRepository) {
				repo.EXPECT().GetRoomTypes(ctx).Return([]*model.RoomType{}, nil).Once()
			},
			want: []*model.RoomType{},
		},
		{
			name: "propagates the repository error",
			expectations: func(repo *mock.MockRoomRepository) {
				repo.EXPECT().GetRoomTypes(ctx).Return(nil, errDefault).Once()
			},
			wantErr: errDefault,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := newTestService(t, tt.expectations).FindAllRoomTypes(ctx)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, got)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRoomService_CreateRooms(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	rooms := []*model.RoomItem{
		{ID: "r1", HotelID: "h1", TypeID: 1, Label: "101", IsAvailable: true},
		{ID: "r2", HotelID: "h1", TypeID: 2, Label: "102", IsAvailable: false},
	}

	tests := []struct {
		name         string
		rooms        []*model.RoomItem
		expectations func(repo *mock.MockRoomRepository)
		wantErr      error
	}{
		{
			name:  "stores the whole batch through the repository",
			rooms: rooms,
			expectations: func(repo *mock.MockRoomRepository) {
				repo.EXPECT().PutRooms(ctx, rooms).Return(nil).Once()
			},
		},
		{
			name:  "passes an empty batch through unchanged",
			rooms: []*model.RoomItem{},
			expectations: func(repo *mock.MockRoomRepository) {
				repo.EXPECT().PutRooms(ctx, []*model.RoomItem{}).Return(nil).Once()
			},
		},
		{
			name:  "propagates the repository error",
			rooms: rooms,
			expectations: func(repo *mock.MockRoomRepository) {
				repo.EXPECT().PutRooms(ctx, rooms).Return(errDefault).Once()
			},
			wantErr: errDefault,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := newTestService(t, tt.expectations).CreateRooms(ctx, tt.rooms)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestRoomService_RemoveAllRooms(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	tests := []struct {
		name         string
		expectations func(repo *mock.MockRoomRepository)
		wantErr      error
	}{
		{
			name: "removes everything through the repository",
			expectations: func(repo *mock.MockRoomRepository) {
				repo.EXPECT().DeleteAllRooms(ctx).Return(nil).Once()
			},
		},
		{
			name: "propagates the repository error",
			expectations: func(repo *mock.MockRoomRepository) {
				repo.EXPECT().DeleteAllRooms(ctx).Return(errDefault).Once()
			},
			wantErr: errDefault,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := newTestService(t, tt.expectations).RemoveAllRooms(ctx)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
