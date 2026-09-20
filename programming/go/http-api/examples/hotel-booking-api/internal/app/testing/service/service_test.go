package service

import (
	"context"
	"errors"
	"testing"

	modelTesting "hotel-booking-api/internal/app/testing/model"
	"hotel-booking-api/internal/app/testing/service/mock"
	modelHotel "hotel-booking-api/internal/hotel/model"
	modelRoom "hotel-booking-api/internal/room/model"

	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var errDefault = errors.New("error default")

type serviceMocks struct {
	hotels       *mock.MockHotelService
	rooms        *mock.MockRoomService
	reservations *mock.MockReservationService
}

func newTestService(t *testing.T, expectations func(m serviceMocks)) *testingService {
	t.Helper()

	m := serviceMocks{
		hotels:       mock.NewMockHotelService(t),
		rooms:        mock.NewMockRoomService(t),
		reservations: mock.NewMockReservationService(t),
	}
	expectations(m)

	return New(m.hotels, m.rooms, m.reservations)
}

func expectResetData(ctx context.Context, m serviceMocks) {
	reservations := m.reservations.EXPECT().RemoveAllReservations(ctx).Return(nil).Once()
	rooms := m.rooms.EXPECT().RemoveAllRooms(ctx).Return(nil).Once()
	hotels := m.hotels.EXPECT().RemoveAllHotels(ctx).Return(nil).Once()

	rooms.NotBefore(reservations)
	hotels.NotBefore(rooms)
}

func hotelsMatcher(count int) any {
	return testifymock.MatchedBy(func(hotels []*modelHotel.HotelItem) bool {
		if len(hotels) != count {
			return false
		}
		for i, hotel := range hotels {
			switch {
			case hotel.ID == "":
				return false
			case hotel.Name != hotelName(i+1):
				return false
			case hotel.Address == "" || hotel.Phone == "":
				return false
			}
		}
		return true
	})
}

func roomsMatcher(count int, roomTypes []*modelRoom.RoomType) any {
	return testifymock.MatchedBy(func(rooms []*modelRoom.RoomItem) bool {
		if len(rooms) != count {
			return false
		}
		for i, room := range rooms {
			roomType := roomTypes[i%len(roomTypes)]
			switch {
			case room.ID == "" || room.HotelID == "":
				return false
			case !room.IsAvailable:
				return false
			case room.RoomType != roomType || room.TypeID != roomType.ID:
				return false
			}
		}
		return true
	})
}

func TestTestingService_ResetTestData(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	tests := []struct {
		name         string
		expectations func(m serviceMocks)
		wantErr      error
		wantErrText  string
	}{
		{
			name: "removes reservations, then rooms, then hotels",
			expectations: func(m serviceMocks) {
				expectResetData(ctx, m)
			},
		},
		{
			name: "stops at the reservations and wraps the error",
			expectations: func(m serviceMocks) {
				m.reservations.EXPECT().RemoveAllReservations(ctx).Return(errDefault).Once()
			},
			wantErr:     errDefault,
			wantErrText: "remove reservations:",
		},
		{
			name: "stops at the rooms and wraps the error",
			expectations: func(m serviceMocks) {
				m.reservations.EXPECT().RemoveAllReservations(ctx).Return(nil).Once()
				m.rooms.EXPECT().RemoveAllRooms(ctx).Return(errDefault).Once()
			},
			wantErr:     errDefault,
			wantErrText: "remove rooms:",
		},
		{
			name: "stops at the hotels and wraps the error",
			expectations: func(m serviceMocks) {
				m.reservations.EXPECT().RemoveAllReservations(ctx).Return(nil).Once()
				m.rooms.EXPECT().RemoveAllRooms(ctx).Return(nil).Once()
				m.hotels.EXPECT().RemoveAllHotels(ctx).Return(errDefault).Once()
			},
			wantErr:     errDefault,
			wantErrText: "remove hotels:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := newTestService(t, tt.expectations).ResetTestData(ctx)

			switch {
			case tt.wantErr != nil && tt.wantErrText != "":
				require.ErrorIs(t, err, tt.wantErr)
				require.ErrorContains(t, err, tt.wantErrText)
			case tt.wantErr != nil:
				require.ErrorIs(t, err, tt.wantErr)
			default:
				require.NoError(t, err)
			}
		})
	}
}

func TestTestingService_SeedTestData(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	roomTypes := []*modelRoom.RoomType{
		{ID: 1, Caption: "Double", Capacity: 2},
		{ID: 2, Caption: "Suite", Capacity: 4},
		{ID: 3, Caption: "Single", Capacity: 1},
	}

	tests := []struct {
		name         string
		params       modelTesting.SeedParams
		expectations func(m serviceMocks)
		verify       func(t *testing.T, result *modelTesting.SeedResult)
		wantErr      error
		wantErrText  string
	}{
		{
			name:   "seeds the requested hotels and rooms and reports what was created",
			params: modelTesting.SeedParams{Hotels: 2, RoomsPerHotel: len(roomTypes)},
			expectations: func(m serviceMocks) {
				expectResetData(ctx, m)
				m.rooms.EXPECT().FindAllRoomTypes(ctx).Return(roomTypes, nil).Once()
				m.hotels.EXPECT().CreateHotels(ctx, hotelsMatcher(2)).Return(nil).Once()
				m.rooms.EXPECT().CreateRooms(ctx, roomsMatcher(2*len(roomTypes), roomTypes)).Return(nil).Once()
			},
			verify: func(t *testing.T, result *modelTesting.SeedResult) {
				t.Helper()

				require.Len(t, result.Hotels, 2)
				require.Len(t, result.Rooms, 6)
				assert.Equal(t, roomTypes, result.RoomTypes)

				assert.Equal(t, "1 Mercury Serenity", result.Hotels[0].Name)
				assert.Equal(t, "2 Venus Serenity", result.Hotels[1].Name)
				assert.Equal(t, "Address 1, Test City", result.Hotels[0].Address)
				assert.Equal(t, "+44 1234 900001", result.Hotels[0].Phone)

				hotelIDs := make(map[string]struct{}, len(result.Hotels))
				for _, hotel := range result.Hotels {
					hotelIDs[hotel.ID] = struct{}{}
				}
				for _, room := range result.Rooms {
					_, ok := hotelIDs[room.HotelID]
					assert.Truef(t, ok, "room %q references unknown hotel %q", room.ID, room.HotelID)
				}
			},
		},
		{
			name:   "seeds nothing when no hotel is requested",
			params: modelTesting.SeedParams{Hotels: 0, RoomsPerHotel: 0},
			expectations: func(m serviceMocks) {
				expectResetData(ctx, m)
				m.rooms.EXPECT().FindAllRoomTypes(ctx).Return(roomTypes, nil).Once()
				m.hotels.EXPECT().CreateHotels(ctx, hotelsMatcher(0)).Return(nil).Once()
				m.rooms.EXPECT().CreateRooms(ctx, roomsMatcher(0, roomTypes)).Return(nil).Once()
			},
			verify: func(t *testing.T, result *modelTesting.SeedResult) {
				t.Helper()

				assert.Empty(t, result.Hotels)
				assert.Empty(t, result.Rooms)
				assert.Equal(t, roomTypes, result.RoomTypes)
			},
		},
		{
			name:   "wraps a failure while resetting the previous data",
			params: modelTesting.SeedParams{Hotels: 1, RoomsPerHotel: 1},
			expectations: func(m serviceMocks) {
				m.reservations.EXPECT().RemoveAllReservations(ctx).Return(errDefault).Once()
			},
			wantErr:     errDefault,
			wantErrText: "reset test data: remove reservations:",
		},
		{
			name:   "stops when no room type is seeded",
			params: modelTesting.SeedParams{Hotels: 1, RoomsPerHotel: 1},
			expectations: func(m serviceMocks) {
				expectResetData(ctx, m)
				m.rooms.EXPECT().FindAllRoomTypes(ctx).Return([]*modelRoom.RoomType{}, nil).Once()
			},
			wantErr: modelTesting.ErrNoRoomTypes,
		},
		{
			name:   "wraps a room type lookup failure",
			params: modelTesting.SeedParams{Hotels: 1, RoomsPerHotel: 1},
			expectations: func(m serviceMocks) {
				expectResetData(ctx, m)
				m.rooms.EXPECT().FindAllRoomTypes(ctx).Return(nil, errDefault).Once()
			},
			wantErr:     errDefault,
			wantErrText: "find room types:",
		},
		{
			name:   "wraps a hotel creation failure",
			params: modelTesting.SeedParams{Hotels: 1, RoomsPerHotel: 1},
			expectations: func(m serviceMocks) {
				expectResetData(ctx, m)
				m.rooms.EXPECT().FindAllRoomTypes(ctx).Return(roomTypes, nil).Once()
				m.hotels.EXPECT().CreateHotels(ctx, testifymock.Anything).Return(errDefault).Once()
			},
			wantErr:     errDefault,
			wantErrText: "create hotels:",
		},
		{
			name:   "wraps a room creation failure",
			params: modelTesting.SeedParams{Hotels: 1, RoomsPerHotel: 1},
			expectations: func(m serviceMocks) {
				expectResetData(ctx, m)
				m.rooms.EXPECT().FindAllRoomTypes(ctx).Return(roomTypes, nil).Once()
				m.hotels.EXPECT().CreateHotels(ctx, testifymock.Anything).Return(nil).Once()
				m.rooms.EXPECT().CreateRooms(ctx, testifymock.Anything).Return(errDefault).Once()
			},
			wantErr:     errDefault,
			wantErrText: "create rooms:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := newTestService(t, tt.expectations).SeedTestData(ctx, tt.params)

			switch {
			case tt.wantErr != nil && tt.wantErrText != "":
				require.ErrorIs(t, err, tt.wantErr)
				require.ErrorContains(t, err, tt.wantErrText)
			case tt.wantErr != nil:
				require.ErrorIs(t, err, tt.wantErr)
			case tt.wantErrText != "":
				require.ErrorContains(t, err, tt.wantErrText)
			default:
				require.NoError(t, err)
			}

			if err != nil {
				require.Nil(t, result)
				return
			}

			require.NotNil(t, result)
			if tt.verify != nil {
				tt.verify(t, result)
			}
		})
	}
}
