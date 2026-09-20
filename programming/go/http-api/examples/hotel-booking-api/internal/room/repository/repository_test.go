package repository

import (
	"context"
	"database/sql/driver"
	"errors"
	"regexp"
	"testing"
	"time"

	modelReservation "hotel-booking-api/internal/reservation/model"
	modelRoom "hotel-booking-api/internal/room/model"

	"hotel-booking-api/internal/helper/pagination"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testQueryTimeout = time.Second
)

var errDefault = errors.New("error default")

var roomColumns = []string{
	"id", "hotel_id", "room_label", "is_available",
	"room_type_id", "caption", "capacity", "name", "address",
}

type roomFixture struct {
	ID           string
	HotelID      string
	Label        string
	Available    bool
	TypeID       int
	Caption      string
	Capacity     int
	HotelName    string
	HotelAddress string
}

func (f roomFixture) values() []driver.Value {
	return []driver.Value{
		f.ID, f.HotelID, f.Label, f.Available,
		f.TypeID, f.Caption, f.Capacity, f.HotelName, f.HotelAddress,
	}
}

func (f roomFixture) item() *modelRoom.RoomItem {
	return &modelRoom.RoomItem{
		ID:          f.ID,
		HotelID:     f.HotelID,
		Label:       f.Label,
		IsAvailable: f.Available,
		TypeID:      f.TypeID,
		RoomType: &modelRoom.RoomType{
			ID:       f.TypeID,
			Caption:  f.Caption,
			Capacity: f.Capacity,
		},
		RoomItemHotelDetails: &modelRoom.RoomItemHotelDetails{
			Name:    f.HotelName,
			Address: f.HotelAddress,
		},
	}
}

func roomFixtureRows(fixtures ...roomFixture) *sqlmock.Rows {
	rows := sqlmock.NewRows(roomColumns)
	for _, f := range fixtures {
		rows.AddRow(f.values()...)
	}
	return rows
}

func newTestRepository(t *testing.T, expectations func(mock sqlmock.Sqlmock)) *roomRepository {
	t.Helper()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	t.Cleanup(func() { _ = db.Close() })
	t.Cleanup(func() { assert.NoError(t, mock.ExpectationsWereMet()) })

	expectations(mock)

	return New(db, testQueryTimeout)
}

func roomInsertArgs(rooms ...*modelRoom.RoomItem) []driver.Value {
	var (
		ids          = make([]string, len(rooms))
		hotelIDs     = make([]string, len(rooms))
		typeIDs      = make([]int, len(rooms))
		labels       = make([]string, len(rooms))
		availability = make([]bool, len(rooms))
	)

	for i, room := range rooms {
		ids[i] = room.ID
		hotelIDs[i] = room.HotelID
		typeIDs[i] = room.TypeID
		labels[i] = room.Label
		availability[i] = room.IsAvailable
	}

	return []driver.Value{
		pq.Array(ids),
		pq.Array(hotelIDs),
		pq.Array(typeIDs),
		pq.Array(labels),
		pq.Array(availability),
	}
}

func TestRoomRepository_GetAvailableRooms(t *testing.T) {
	t.Parallel()

	fixtures := []roomFixture{
		{ID: "r1", HotelID: "h1", Label: "101", Available: true, TypeID: 1, Caption: "Double", Capacity: 2, HotelName: "Grand Hotel", HotelAddress: "1 Main St"},
		{ID: "r2", HotelID: "h1", Label: "102", Available: true, TypeID: 1, Caption: "Double", Capacity: 2, HotelName: "Grand Hotel", HotelAddress: "1 Main St"},
		{ID: "r3", HotelID: "h1", Label: "103", Available: true, TypeID: 2, Caption: "Suite", Capacity: 4, HotelName: "Grand Hotel", HotelAddress: "1 Main St"},
	}

	nextPage, err := pagination.Encode(fixtures[1].ID)
	require.NoError(t, err)

	baseParams := modelRoom.FindAvailableRoomsParams{
		CheckInDate:  "2026-10-01",
		CheckOutDate: "2026-10-03",
		GuestsCount:  2,
		HotelID:      "h1",
		PageSize:     2,
	}

	cursorParams := modelRoom.FindAvailableRoomsParams{
		CheckInDate:  "2026-11-01",
		CheckOutDate: "2026-11-05",
		GuestsCount:  4,
		HotelID:      "h3",
		AfterID:      "r1",
		PageSize:     1,
	}

	selectArgs := func(p modelRoom.FindAvailableRoomsParams) []driver.Value {
		return []driver.Value{
			p.GuestsCount,
			p.HotelID,
			p.AfterID,
			int(modelReservation.ReservationStatusCancelled),
			p.CheckInDate,
			p.CheckOutDate,
			p.PageSize + 1,
		}
	}

	tests := []struct {
		name         string
		params       modelRoom.FindAvailableRoomsParams
		expectations func(mock sqlmock.Sqlmock)
		want         *modelRoom.RoomsPage
		wantErr      error
		wantErrText  string
	}{
		{
			name:   "returns the rooms of the page without a next page token",
			params: baseParams,
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(selectAvailableRoomsQuery)).
					WithArgs(selectArgs(baseParams)...).
					WillReturnRows(roomFixtureRows(fixtures[0], fixtures[1]))
			},
			want: &modelRoom.RoomsPage{
				Rooms:    []*modelRoom.RoomItem{fixtures[0].item(), fixtures[1].item()},
				PageSize: 2,
			},
		},
		{
			name:   "trims the extra row and returns a next page token",
			params: baseParams,
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(selectAvailableRoomsQuery)).
					WithArgs(selectArgs(baseParams)...).
					WillReturnRows(roomFixtureRows(fixtures[0], fixtures[1], fixtures[2]))
			},
			want: &modelRoom.RoomsPage{
				Rooms:    []*modelRoom.RoomItem{fixtures[0].item(), fixtures[1].item()},
				PageSize: 2,
				NextPage: nextPage,
			},
		},
		{
			name:   "passes the cursor and the hotel filter through",
			params: cursorParams,
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(selectAvailableRoomsQuery)).
					WithArgs(selectArgs(cursorParams)...).
					WillReturnRows(roomFixtureRows(fixtures[0]))
			},
			want: &modelRoom.RoomsPage{
				Rooms:    []*modelRoom.RoomItem{fixtures[0].item()},
				PageSize: 1,
			},
		},
		{
			name:   "returns an empty page when nothing is available",
			params: baseParams,
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(selectAvailableRoomsQuery)).
					WithArgs(selectArgs(baseParams)...).
					WillReturnRows(roomFixtureRows())
			},
			want: &modelRoom.RoomsPage{Rooms: []*modelRoom.RoomItem{}, PageSize: 2},
		},
		{
			name:   "wraps query errors",
			params: baseParams,
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(selectAvailableRoomsQuery)).
					WithArgs(selectArgs(baseParams)...).
					WillReturnError(errDefault)
			},
			wantErr:     errDefault,
			wantErrText: "find available rooms:",
		},
		{
			name:   "wraps scan errors",
			params: baseParams,
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(selectAvailableRoomsQuery)).
					WithArgs(selectArgs(baseParams)...).
					WillReturnRows(sqlmock.NewRows(roomColumns).
						AddRow("r1", "h1", nil, true, 1, "Double", 2, "Grand Hotel", "1 Main St"))
			},
			wantErrText: "scan room:",
		},
		{
			name:   "wraps iteration errors",
			params: baseParams,
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(selectAvailableRoomsQuery)).
					WithArgs(selectArgs(baseParams)...).
					WillReturnRows(roomFixtureRows(fixtures[0]).RowError(0, errDefault))
			},
			wantErrText: "iterate rooms:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := newTestRepository(t, tt.expectations).GetAvailableRooms(context.Background(), tt.params)

			switch {
			case tt.wantErr != nil:
				require.ErrorIs(t, err, tt.wantErr)
			case tt.wantErrText != "":
				require.ErrorContains(t, err, tt.wantErrText)
			default:
				require.NoError(t, err)
			}

			if tt.wantErr != nil || tt.wantErrText != "" {
				require.Nil(t, got)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRoomRepository_GetRoomTypes(t *testing.T) {
	t.Parallel()

	roomTypes := []*modelRoom.RoomType{
		{ID: 1, Caption: "Double", Capacity: 2},
		{ID: 2, Caption: "Suite", Capacity: 4},
	}

	roomTypeRows := func(types ...*modelRoom.RoomType) *sqlmock.Rows {
		rows := sqlmock.NewRows([]string{"id", "caption", "capacity"})
		for _, rt := range types {
			rows.AddRow(rt.ID, rt.Caption, rt.Capacity)
		}
		return rows
	}

	tests := []struct {
		name         string
		expectations func(mock sqlmock.Sqlmock)
		want         []*modelRoom.RoomType
		wantErr      error
		wantErrText  string
	}{
		{
			name: "returns all room types",
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(selectRoomTypesQuery)).
					WillReturnRows(roomTypeRows(roomTypes...))
			},
			want: roomTypes,
		},
		{
			name: "returns an empty list when there are no room types",
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(selectRoomTypesQuery)).
					WillReturnRows(roomTypeRows())
			},
			want: []*modelRoom.RoomType{},
		},
		{
			name: "wraps query errors",
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(selectRoomTypesQuery)).
					WillReturnError(errDefault)
			},
			wantErr:     errDefault,
			wantErrText: "get room types:",
		},
		{
			name: "wraps scan errors",
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(selectRoomTypesQuery)).
					WillReturnRows(sqlmock.NewRows([]string{"id", "caption", "capacity"}).
						AddRow(1, nil, 2))
			},
			wantErrText: "scan room type:",
		},
		{
			name: "wraps iteration errors",
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(selectRoomTypesQuery)).
					WillReturnRows(roomTypeRows(roomTypes[0]).RowError(0, errDefault))
			},
			wantErrText: "iterate room types:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := newTestRepository(t, tt.expectations).GetRoomTypes(context.Background())

			switch {
			case tt.wantErr != nil:
				require.ErrorIs(t, err, tt.wantErr)
			case tt.wantErrText != "":
				require.ErrorContains(t, err, tt.wantErrText)
			default:
				require.NoError(t, err)
			}

			if tt.wantErr != nil || tt.wantErrText != "" {
				require.Nil(t, got)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRoomRepository_PutRooms(t *testing.T) {
	t.Parallel()

	rooms := []*modelRoom.RoomItem{
		{ID: "r1", HotelID: "h1", TypeID: 1, Label: "101", IsAvailable: true},
		{ID: "r2", HotelID: "h1", TypeID: 2, Label: "102", IsAvailable: false},
	}

	tests := []struct {
		name         string
		rooms        []*modelRoom.RoomItem
		expectations func(mock sqlmock.Sqlmock)
		wantErr      error
		wantErrText  string
	}{
		{
			name:  "inserts all rooms in a single statement",
			rooms: rooms,
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta(insertRoomsQuery)).
					WithArgs(roomInsertArgs(rooms...)...).
					WillReturnResult(sqlmock.NewResult(0, 2))
			},
		},
		{
			name:         "skips the statement when there is nothing to insert",
			rooms:        nil,
			expectations: func(mock sqlmock.Sqlmock) {},
		},
		{
			name:  "wraps exec errors",
			rooms: rooms,
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta(insertRoomsQuery)).
					WithArgs(roomInsertArgs(rooms...)...).
					WillReturnError(errDefault)
			},
			wantErr:     errDefault,
			wantErrText: "insert rooms:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := newTestRepository(t, tt.expectations).PutRooms(context.Background(), tt.rooms)

			switch {
			case tt.wantErr != nil:
				require.ErrorIs(t, err, tt.wantErr)
			case tt.wantErrText != "":
				require.ErrorContains(t, err, tt.wantErrText)
			default:
				require.NoError(t, err)
			}
		})
	}
}

func TestRoomRepository_DeleteAllRooms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		expectations func(mock sqlmock.Sqlmock)
		wantErr      error
		wantErrText  string
	}{
		{
			name: "deletes every room",
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta(deleteAllRoomsQuery)).
					WillReturnResult(sqlmock.NewResult(0, 3))
			},
		},
		{
			name: "wraps exec errors",
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta(deleteAllRoomsQuery)).
					WillReturnError(errDefault)
			},
			wantErr:     errDefault,
			wantErrText: "delete all rooms:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := newTestRepository(t, tt.expectations).DeleteAllRooms(context.Background())

			switch {
			case tt.wantErr != nil:
				require.ErrorIs(t, err, tt.wantErr)
			case tt.wantErrText != "":
				require.ErrorContains(t, err, tt.wantErrText)
			default:
				require.NoError(t, err)
			}
		})
	}
}
