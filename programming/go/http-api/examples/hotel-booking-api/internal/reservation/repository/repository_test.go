package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"regexp"
	"testing"
	"time"

	"hotel-booking-api/internal/reservation/model"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
	"github.com/lib/pq/pqerror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testQueryTimeout = time.Second
)

var errDefault = errors.New("error default")

const (
	testHotelID      = "h1"
	testCheckInDate  = "2026-10-01"
	testCheckOutDate = "2026-10-03"
	testReference    = "REF-1"
)

func newTestRepository(t *testing.T, expectations func(mock sqlmock.Sqlmock)) *reservationRepository {
	t.Helper()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	t.Cleanup(func() { _ = db.Close() })
	t.Cleanup(func() { assert.NoError(t, mock.ExpectationsWereMet()) })

	expectations(mock)

	return New(db, testQueryTimeout)
}

func storedReservation(guestsCount int) *model.ReservationItem {
	return &model.ReservationItem{
		ID:            "res-1",
		CheckInDate:   testCheckInDate,
		CheckOutDate:  testCheckOutDate,
		GuestFullName: "Jane Doe",
		GuestEmail:    "jane@example.com",
		GuestsCount:   guestsCount,
		Reference:     testReference,
		Status:        model.ReservationStatusPending,
		HotelID:       testHotelID,
		ReservationHotelDetails: &model.ReservationHotelDetails{
			Name:    "Grand Hotel",
			Address: "1 Main St",
		},
	}
}

func withRooms(reservation *model.ReservationItem, rooms ...*model.ReservationRoomDetails) *model.ReservationItem {
	clone := *reservation
	clone.Rooms = rooms
	return &clone
}

func bookingRequest() model.ReservationItem {
	return model.ReservationItem{
		Reference:     testReference,
		HotelID:       testHotelID,
		GuestFullName: "Jane Doe",
		GuestEmail:    "jane@example.com",
		GuestsCount:   2,
		CheckInDate:   testCheckInDate,
		CheckOutDate:  testCheckOutDate,
		Status:        model.ReservationStatusPending,
	}
}

var reservationColumns = []string{
	"id", "check_in_date", "check_out_date", "guest_full_name", "guest_email",
	"guests_count", "reference", "reservation_status", "hotel_id", "name", "address",
}

func reservationRows(reservations ...*model.ReservationItem) *sqlmock.Rows {
	rows := sqlmock.NewRows(reservationColumns)
	for _, r := range reservations {
		rows.AddRow(
			r.ID, r.CheckInDate, r.CheckOutDate, r.GuestFullName, r.GuestEmail,
			r.GuestsCount, r.Reference, int(r.Status), r.HotelID,
			r.Name, r.Address,
		)
	}
	return rows
}

func reservationRoomRowValues(reservationID string, room *model.ReservationRoomDetails) []driver.Value {
	return []driver.Value{
		reservationID, room.RoomID, room.GuestsCount, room.Label, room.RoomTypeCaption, room.RoomTypeCapacity,
	}
}

func reservationRoomRows(rows ...[]driver.Value) *sqlmock.Rows {
	result := sqlmock.NewRows([]string{
		"reservation_id", "room_id", "guests_count", "room_label", "room_type_caption", "room_type_capacity",
	})
	for _, row := range rows {
		result.AddRow(row...)
	}
	return result
}

func candidateRoomRows(rows ...[]driver.Value) *sqlmock.Rows {
	result := sqlmock.NewRows([]string{"id", "room_label", "caption", "capacity"})
	for _, row := range rows {
		result.AddRow(row...)
	}
	return result
}

func expectHotelExists(mock sqlmock.Sqlmock, exists bool) {
	mock.ExpectQuery(regexp.QuoteMeta(existsHotelQuery)).
		WithArgs(testHotelID).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(exists))
}

func expectInsertReservation(mock sqlmock.Sqlmock, guestsCount int) *model.ReservationItem {
	stored := storedReservation(guestsCount)

	mock.ExpectQuery(regexp.QuoteMeta(insertReservationQuery)).
		WithArgs(
			testReference,
			testHotelID,
			"Jane Doe",
			"jane@example.com",
			guestsCount,
			testCheckInDate,
			testCheckOutDate,
			int(model.ReservationStatusPending),
		).
		WillReturnRows(reservationRows(stored))

	return stored
}

func TestReservationRepository_PutReservation(t *testing.T) {
	t.Parallel()

	lockArgs := []driver.Value{
		testHotelID, int(model.ReservationStatusCancelled), testCheckInDate, testCheckOutDate,
	}

	tests := []struct {
		name         string
		mutate       func(*model.ReservationItem)
		expectations func(mock sqlmock.Sqlmock)
		want         *model.ReservationItem
		wantErr      error
		wantErrText  string
	}{
		{
			name:   "allocates the requested rooms and commits the booking",
			mutate: func(r *model.ReservationItem) { r.RoomIDs = []string{"r1"} },
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				expectHotelExists(mock, true)
				mock.ExpectQuery(regexp.QuoteMeta(countRequestedRoomsQuery)).
					WithArgs(testHotelID, pq.Array([]string{"r1"})).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
				mock.ExpectQuery(regexp.QuoteMeta(lockRequestedRoomsQuery)).
					WithArgs(testHotelID, pq.Array([]string{"r1"}), int(model.ReservationStatusCancelled), testCheckInDate, testCheckOutDate).
					WillReturnRows(candidateRoomRows([]driver.Value{"r1", "101", "Double", 2}))
				stored := expectInsertReservation(mock, 2)
				mock.ExpectExec(regexp.QuoteMeta(insertReservationRoomsQuery)).
					WithArgs(pq.Array([]string{stored.ID}), pq.Array([]string{"r1"}), pq.Array([]int{2})).
					WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit()
			},
			want: withRooms(storedReservation(2), &model.ReservationRoomDetails{
				RoomID: "r1", Label: "101", RoomTypeCaption: "Double", RoomTypeCapacity: 2, GuestsCount: 2,
			}),
		},
		{
			name:   "picks the smallest single room that fits the whole party",
			mutate: func(r *model.ReservationItem) { r.GuestsCount = 3 },
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				expectHotelExists(mock, true)
				mock.ExpectQuery(regexp.QuoteMeta(lockAvailableRoomsQuery)).
					WithArgs(lockArgs...).
					WillReturnRows(candidateRoomRows(
						[]driver.Value{"r1", "101", "Double", 2},
						[]driver.Value{"r2", "201", "Suite", 5},
					))
				stored := expectInsertReservation(mock, 3)
				mock.ExpectExec(regexp.QuoteMeta(insertReservationRoomsQuery)).
					WithArgs(pq.Array([]string{stored.ID}), pq.Array([]string{"r2"}), pq.Array([]int{3})).
					WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit()
			},
			want: withRooms(storedReservation(3), &model.ReservationRoomDetails{
				RoomID: "r2", Label: "201", RoomTypeCaption: "Suite", RoomTypeCapacity: 5, GuestsCount: 3,
			}),
		},
		{
			name:   "distributes the party over several rooms when no single room fits",
			mutate: func(r *model.ReservationItem) { r.GuestsCount = 7 },
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				expectHotelExists(mock, true)
				mock.ExpectQuery(regexp.QuoteMeta(lockAvailableRoomsQuery)).
					WithArgs(lockArgs...).
					WillReturnRows(candidateRoomRows(
						[]driver.Value{"r1", "101", "Quad", 4},
						[]driver.Value{"r2", "102", "Triple", 3},
						[]driver.Value{"r3", "103", "Double", 2},
					))
				stored := expectInsertReservation(mock, 7)
				mock.ExpectExec(regexp.QuoteMeta(insertReservationRoomsQuery)).
					WithArgs(
						pq.Array([]string{stored.ID, stored.ID}),
						pq.Array([]string{"r1", "r2"}),
						pq.Array([]int{4, 3}),
					).
					WillReturnResult(sqlmock.NewResult(2, 2))
				mock.ExpectCommit()
			},
			want: withRooms(storedReservation(7),
				&model.ReservationRoomDetails{RoomID: "r1", Label: "101", RoomTypeCaption: "Quad", RoomTypeCapacity: 4, GuestsCount: 4},
				&model.ReservationRoomDetails{RoomID: "r2", Label: "102", RoomTypeCaption: "Triple", RoomTypeCapacity: 3, GuestsCount: 3},
			),
		},
		{
			name:   "fails when the available rooms cannot host the party",
			mutate: func(r *model.ReservationItem) { r.GuestsCount = 10 },
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				expectHotelExists(mock, true)
				mock.ExpectQuery(regexp.QuoteMeta(lockAvailableRoomsQuery)).
					WithArgs(lockArgs...).
					WillReturnRows(candidateRoomRows(
						[]driver.Value{"r1", "101", "Quad", 4},
						[]driver.Value{"r2", "102", "Triple", 3},
					))
				mock.ExpectRollback()
			},
			wantErr: model.ErrInsufficientCapacity,
		},
		{
			name: "fails when the hotel does not exist",
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				expectHotelExists(mock, false)
				mock.ExpectRollback()
			},
			wantErr: model.ErrHotelNotFound,
		},
		{
			name:   "fails when one of the requested rooms is unknown",
			mutate: func(r *model.ReservationItem) { r.RoomIDs = []string{"r2", "r1"} },
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				expectHotelExists(mock, true)
				mock.ExpectQuery(regexp.QuoteMeta(countRequestedRoomsQuery)).
					WithArgs(testHotelID, pq.Array([]string{"r1", "r2"})).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
				mock.ExpectRollback()
			},
			wantErr: model.ErrRoomNotFound,
		},
		{
			name:   "fails when a requested room is taken for the dates",
			mutate: func(r *model.ReservationItem) { r.RoomIDs = []string{"r1"} },
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				expectHotelExists(mock, true)
				mock.ExpectQuery(regexp.QuoteMeta(countRequestedRoomsQuery)).
					WithArgs(testHotelID, pq.Array([]string{"r1"})).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
				mock.ExpectQuery(regexp.QuoteMeta(lockRequestedRoomsQuery)).
					WithArgs(testHotelID, pq.Array([]string{"r1"}), int(model.ReservationStatusCancelled), testCheckInDate, testCheckOutDate).
					WillReturnRows(candidateRoomRows())
				mock.ExpectRollback()
			},
			wantErr: model.ErrRoomUnavailable,
		},
		{
			name:   "reports a taken booking reference on a unique violation",
			mutate: func(r *model.ReservationItem) { r.RoomIDs = []string{"r1"} },
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				expectHotelExists(mock, true)
				mock.ExpectQuery(regexp.QuoteMeta(countRequestedRoomsQuery)).
					WithArgs(testHotelID, pq.Array([]string{"r1"})).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
				mock.ExpectQuery(regexp.QuoteMeta(lockRequestedRoomsQuery)).
					WithArgs(testHotelID, pq.Array([]string{"r1"}), int(model.ReservationStatusCancelled), testCheckInDate, testCheckOutDate).
					WillReturnRows(candidateRoomRows([]driver.Value{"r1", "101", "Double", 2}))
				mock.ExpectQuery(regexp.QuoteMeta(insertReservationQuery)).
					WillReturnError(&pq.Error{Code: pqerror.UniqueViolation})
				mock.ExpectRollback()
			},
			wantErr: model.ErrReferenceExists,
		},
		{
			name: "fails when the transaction cannot be started",
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin().WillReturnError(errDefault)
			},
			wantErr:     errDefault,
			wantErrText: "begin transaction:",
		},
		{
			name:   "wraps insert errors",
			mutate: func(r *model.ReservationItem) { r.RoomIDs = []string{"r1"} },
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				expectHotelExists(mock, true)
				mock.ExpectQuery(regexp.QuoteMeta(countRequestedRoomsQuery)).
					WithArgs(testHotelID, pq.Array([]string{"r1"})).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
				mock.ExpectQuery(regexp.QuoteMeta(lockRequestedRoomsQuery)).
					WithArgs(testHotelID, pq.Array([]string{"r1"}), int(model.ReservationStatusCancelled), testCheckInDate, testCheckOutDate).
					WillReturnRows(candidateRoomRows([]driver.Value{"r1", "101", "Double", 2}))
				mock.ExpectQuery(regexp.QuoteMeta(insertReservationQuery)).
					WillReturnError(errDefault)
				mock.ExpectRollback()
			},
			wantErr:     errDefault,
			wantErrText: "insert reservation:",
		},
		{
			name:   "wraps commit errors",
			mutate: func(r *model.ReservationItem) { r.RoomIDs = []string{"r1"} },
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				expectHotelExists(mock, true)
				mock.ExpectQuery(regexp.QuoteMeta(countRequestedRoomsQuery)).
					WithArgs(testHotelID, pq.Array([]string{"r1"})).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
				mock.ExpectQuery(regexp.QuoteMeta(lockRequestedRoomsQuery)).
					WithArgs(testHotelID, pq.Array([]string{"r1"}), int(model.ReservationStatusCancelled), testCheckInDate, testCheckOutDate).
					WillReturnRows(candidateRoomRows([]driver.Value{"r1", "101", "Double", 2}))
				stored := expectInsertReservation(mock, 2)
				mock.ExpectExec(regexp.QuoteMeta(insertReservationRoomsQuery)).
					WithArgs(pq.Array([]string{stored.ID}), pq.Array([]string{"r1"}), pq.Array([]int{2})).
					WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit().WillReturnError(errDefault)
			},
			wantErr:     errDefault,
			wantErrText: "commit transaction:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			booking := bookingRequest()
			if tt.mutate != nil {
				tt.mutate(&booking)
			}

			got, err := newTestRepository(t, tt.expectations).PutReservation(context.Background(), booking)

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

func TestReservationRepository_GetReservationByReference(t *testing.T) {
	t.Parallel()

	room := &model.ReservationRoomDetails{
		RoomID: "r1", GuestsCount: 2, Label: "101", RoomTypeCaption: "Double", RoomTypeCapacity: 2,
	}
	stored := storedReservation(2)

	tests := []struct {
		name         string
		expectations func(mock sqlmock.Sqlmock)
		want         *model.ReservationItem
		wantErr      error
		wantErrText  string
	}{
		{
			name: "returns the reservation with its allocated rooms",
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(selectByReferenceQuery)).
					WithArgs(testReference).
					WillReturnRows(reservationRows(stored))
				mock.ExpectQuery(regexp.QuoteMeta(reservationRoomsSelect)).
					WithArgs(pq.Array([]string{stored.ID})).
					WillReturnRows(reservationRoomRows(reservationRoomRowValues(stored.ID, room)))
			},
			want: withRooms(stored, room),
		},
		{
			name: "returns nil for an unknown reference",
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(selectByReferenceQuery)).
					WithArgs(testReference).
					WillReturnError(sql.ErrNoRows)
			},
		},
		{
			name: "wraps query errors",
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(selectByReferenceQuery)).
					WithArgs(testReference).
					WillReturnError(errDefault)
			},
			wantErr:     errDefault,
			wantErrText: "get reservation by reference:",
		},
		{
			name: "wraps scan errors",
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(selectByReferenceQuery)).
					WithArgs(testReference).
					WillReturnRows(sqlmock.NewRows(reservationColumns).
						AddRow(nil, testCheckInDate, testCheckOutDate, "Jane Doe", "jane@example.com",
							2, testReference, int(model.ReservationStatusPending), testHotelID, "Grand Hotel", "1 Main St"))
			},
			wantErrText: "get reservation by reference:",
		},
		{
			name: "wraps room loading errors",
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(selectByReferenceQuery)).
					WithArgs(testReference).
					WillReturnRows(reservationRows(stored))
				mock.ExpectQuery(regexp.QuoteMeta(reservationRoomsSelect)).
					WithArgs(pq.Array([]string{stored.ID})).
					WillReturnError(errDefault)
			},
			wantErr:     errDefault,
			wantErrText: "get reservation rooms:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := newTestRepository(t, tt.expectations).GetReservationByReference(context.Background(), testReference)

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

func TestReservationRepository_GetReservationByID(t *testing.T) {
	t.Parallel()

	room := &model.ReservationRoomDetails{
		RoomID: "r1", GuestsCount: 2, Label: "101", RoomTypeCaption: "Double", RoomTypeCapacity: 2,
	}
	stored := storedReservation(2)

	tests := []struct {
		name         string
		expectations func(mock sqlmock.Sqlmock)
		want         *model.ReservationItem
		wantErr      error
		wantErrText  string
	}{
		{
			name: "returns the reservation with its allocated rooms",
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(selectByIDQuery)).
					WithArgs(stored.ID).
					WillReturnRows(reservationRows(stored))
				mock.ExpectQuery(regexp.QuoteMeta(reservationRoomsSelect)).
					WithArgs(pq.Array([]string{stored.ID})).
					WillReturnRows(reservationRoomRows(reservationRoomRowValues(stored.ID, room)))
			},
			want: withRooms(stored, room),
		},
		{
			name: "returns nil for an unknown id",
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(selectByIDQuery)).
					WithArgs(stored.ID).
					WillReturnError(sql.ErrNoRows)
			},
		},
		{
			name: "wraps query errors",
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(selectByIDQuery)).
					WithArgs(stored.ID).
					WillReturnError(errDefault)
			},
			wantErr:     errDefault,
			wantErrText: "get reservation by id:",
		},
		{
			name: "wraps scan errors",
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(selectByIDQuery)).
					WithArgs(stored.ID).
					WillReturnRows(sqlmock.NewRows(reservationColumns).
						AddRow(nil, testCheckInDate, testCheckOutDate, "Jane Doe", "jane@example.com",
							2, testReference, int(model.ReservationStatusPending), testHotelID, "Grand Hotel", "1 Main St"))
			},
			wantErrText: "get reservation by id:",
		},
		{
			name: "wraps room loading errors",
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(selectByIDQuery)).
					WithArgs(stored.ID).
					WillReturnRows(reservationRows(stored))
				mock.ExpectQuery(regexp.QuoteMeta(reservationRoomsSelect)).
					WithArgs(pq.Array([]string{stored.ID})).
					WillReturnError(errDefault)
			},
			wantErr:     errDefault,
			wantErrText: "get reservation rooms:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := newTestRepository(t, tt.expectations).GetReservationByID(context.Background(), stored.ID)

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

func TestReservationRepository_UpdateReservationStatus(t *testing.T) {
	t.Parallel()

	room := &model.ReservationRoomDetails{
		RoomID: "r1", GuestsCount: 2, Label: "101", RoomTypeCaption: "Double", RoomTypeCapacity: 2,
	}

	confirmed := storedReservation(2)
	confirmed.Status = model.ReservationStatusConfirmed

	tests := []struct {
		name         string
		expectations func(mock sqlmock.Sqlmock)
		want         *model.ReservationItem
		wantErr      error
		wantErrText  string
	}{
		{
			name: "updates the status and returns the reservation with its rooms",
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(updateReservationStatusQuery)).
					WithArgs(confirmed.ID, int(model.ReservationStatusConfirmed)).
					WillReturnRows(reservationRows(confirmed))
				mock.ExpectQuery(regexp.QuoteMeta(reservationRoomsSelect)).
					WithArgs(pq.Array([]string{confirmed.ID})).
					WillReturnRows(reservationRoomRows(reservationRoomRowValues(confirmed.ID, room)))
			},
			want: withRooms(confirmed, room),
		},
		{
			name: "reports a missing reservation",
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(updateReservationStatusQuery)).
					WithArgs(confirmed.ID, int(model.ReservationStatusConfirmed)).
					WillReturnError(sql.ErrNoRows)
			},
			wantErr: model.ErrReservationNotFound,
		},
		{
			name: "wraps query errors",
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(updateReservationStatusQuery)).
					WithArgs(confirmed.ID, int(model.ReservationStatusConfirmed)).
					WillReturnError(errDefault)
			},
			wantErr:     errDefault,
			wantErrText: "update reservation status:",
		},
		{
			name: "wraps room loading errors",
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(updateReservationStatusQuery)).
					WithArgs(confirmed.ID, int(model.ReservationStatusConfirmed)).
					WillReturnRows(reservationRows(confirmed))
				mock.ExpectQuery(regexp.QuoteMeta(reservationRoomsSelect)).
					WithArgs(pq.Array([]string{confirmed.ID})).
					WillReturnError(errDefault)
			},
			wantErr:     errDefault,
			wantErrText: "get reservation rooms:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := newTestRepository(t, tt.expectations).
				UpdateReservationStatus(context.Background(), confirmed.ID, model.ReservationStatusConfirmed)

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

func TestReservationRepository_GetAllReservationsByStatus(t *testing.T) {
	t.Parallel()

	room1 := &model.ReservationRoomDetails{
		RoomID: "r1", GuestsCount: 2, Label: "101", RoomTypeCaption: "Double", RoomTypeCapacity: 2,
	}
	room2 := &model.ReservationRoomDetails{
		RoomID: "r2", GuestsCount: 1, Label: "102", RoomTypeCaption: "Single", RoomTypeCapacity: 1,
	}

	first := storedReservation(2)
	second := storedReservation(1)
	second.ID = "res-2"
	second.CheckInDate = "2026-10-05"
	second.CheckOutDate = "2026-10-07"

	statusArg := int(model.ReservationStatusPending)

	tests := []struct {
		name         string
		expectations func(mock sqlmock.Sqlmock)
		want         []*model.ReservationItem
		wantErr      error
		wantErrText  string
	}{
		{
			name: "returns every reservation of the status with its rooms",
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(selectByStatusQuery)).
					WithArgs(statusArg).
					WillReturnRows(reservationRows(first, second))
				mock.ExpectQuery(regexp.QuoteMeta(reservationRoomsSelect)).
					WithArgs(pq.Array([]string{first.ID, second.ID})).
					WillReturnRows(reservationRoomRows(
						reservationRoomRowValues(first.ID, room1),
						reservationRoomRowValues(second.ID, room2),
					))
			},
			want: []*model.ReservationItem{withRooms(first, room1), withRooms(second, room2)},
		},
		{
			name: "returns an empty list when nothing matches",
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(selectByStatusQuery)).
					WithArgs(statusArg).
					WillReturnRows(reservationRows())
			},
			want: []*model.ReservationItem{},
		},
		{
			name: "wraps query errors",
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(selectByStatusQuery)).
					WithArgs(statusArg).
					WillReturnError(errDefault)
			},
			wantErr:     errDefault,
			wantErrText: "get reservations by status:",
		},
		{
			name: "wraps scan errors",
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(selectByStatusQuery)).
					WithArgs(statusArg).
					WillReturnRows(sqlmock.NewRows(reservationColumns).
						AddRow(first.ID, testCheckInDate, testCheckOutDate, nil, "jane@example.com",
							2, testReference, int(model.ReservationStatusPending), testHotelID, "Grand Hotel", "1 Main St"))
			},
			wantErrText: "scan reservation:",
		},
		{
			name: "wraps room loading errors",
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(selectByStatusQuery)).
					WithArgs(statusArg).
					WillReturnRows(reservationRows(first))
				mock.ExpectQuery(regexp.QuoteMeta(reservationRoomsSelect)).
					WithArgs(pq.Array([]string{first.ID})).
					WillReturnError(errDefault)
			},
			wantErr:     errDefault,
			wantErrText: "get reservation rooms:",
		},
		{
			name: "wraps room iteration errors",
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(selectByStatusQuery)).
					WithArgs(statusArg).
					WillReturnRows(reservationRows(first))
				mock.ExpectQuery(regexp.QuoteMeta(reservationRoomsSelect)).
					WithArgs(pq.Array([]string{first.ID})).
					WillReturnRows(reservationRoomRows(reservationRoomRowValues(first.ID, room1)).RowError(0, errDefault))
			},
			wantErr:     errDefault,
			wantErrText: "iterate reservation rooms:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := newTestRepository(t, tt.expectations).
				GetAllReservationsByStatus(context.Background(), model.ReservationStatusPending)

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

func TestReservationRepository_DeleteAllReservations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		expectations func(mock sqlmock.Sqlmock)
		wantErr      error
		wantErrText  string
	}{
		{
			name: "deletes every reservation",
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta(deleteAllReservationsQuery)).
					WillReturnResult(sqlmock.NewResult(0, 4))
			},
		},
		{
			name: "wraps exec errors",
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta(deleteAllReservationsQuery)).
					WillReturnError(errDefault)
			},
			wantErr:     errDefault,
			wantErrText: "delete all reservations:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := newTestRepository(t, tt.expectations).DeleteAllReservations(context.Background())

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
