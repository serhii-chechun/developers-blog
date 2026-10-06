package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"time"

	"hotel-booking-api/internal/reservation/model"

	"github.com/lib/pq"
	"github.com/lib/pq/pqerror"
)

type (
	reservationRepository struct {
		db           *sql.DB
		queryTimeout time.Duration
	}

	rowScanner interface {
		Scan(dest ...any) error
	}
)

// New creates a new instance of reservationRepository with the provided sql.DB.
func New(db *sql.DB, queryTimeout time.Duration) *reservationRepository {
	return &reservationRepository{
		db:           db,
		queryTimeout: queryTimeout,
	}
}

type candidateRoom struct {
	ID               string
	Label            string
	RoomTypeCaption  string
	RoomTypeCapacity int
}

func scanReservation(s rowScanner) (*model.ReservationItem, error) {
	var (
		result model.ReservationItem
		hotel  model.ReservationHotelDetails
	)
	if err := s.Scan(
		&result.ID,
		&result.CheckInDate,
		&result.CheckOutDate,
		&result.GuestFullName,
		&result.GuestEmail,
		&result.GuestsCount,
		&result.Reference,
		&result.Status,
		&result.HotelID,
		&hotel.Name,
		&hotel.Address,
	); err != nil {
		return nil, err
	}

	result.ReservationHotelDetails = &hotel

	return &result, nil
}

func (r *reservationRepository) loadRooms(ctx context.Context, reservations []*model.ReservationItem) error {
	if len(reservations) == 0 {
		return nil
	}

	ids := make([]string, len(reservations))
	byID := make(map[string]*model.ReservationItem, len(reservations))
	for i, reservation := range reservations {
		ids[i] = reservation.ID
		byID[reservation.ID] = reservation
	}

	rows, err := r.db.QueryContext(ctx, reservationRoomsSelect, pq.Array(ids))
	if err != nil {
		return fmt.Errorf("get reservation rooms: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			reservationID string
			room          model.ReservationRoomDetails
		)
		if err := rows.Scan(
			&reservationID,
			&room.RoomID,
			&room.GuestsCount,
			&room.Label,
			&room.RoomTypeCaption,
			&room.RoomTypeCapacity,
		); err != nil {
			return fmt.Errorf("scan reservation room: %w", err)
		}

		if reservation, ok := byID[reservationID]; ok {
			reservation.Rooms = append(reservation.Rooms, &room)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate reservation rooms: %w", err)
	}

	return nil
}

// PutReservation allocates the rooms that will hold the party for the requested
// dates and inserts the booking header together with its room allocations.
func (r *reservationRepository) PutReservation(ctx context.Context, res model.ReservationItem) (*model.ReservationItem, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Lock the hotel row to serialize room allocation for concurrent bookings
	// of the same hotel. Without this, two transactions can both pass the
	// availability check for the same rooms before either commits.
	var hotelLocked string
	if err := tx.QueryRowContext(ctx, lockHotelQuery, res.HotelID).Scan(&hotelLocked); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, model.ErrHotelNotFound
		}
		return nil, fmt.Errorf("lock hotel: %w", err)
	}

	allocations, err := allocateReservationRooms(ctx, tx, res)
	if err != nil {
		return nil, err
	}

	result, err := scanReservation(tx.QueryRowContext(ctx, insertReservationQuery,
		res.Reference,
		res.HotelID,
		res.GuestFullName,
		res.GuestEmail,
		res.GuestsCount,
		res.CheckInDate,
		res.CheckOutDate,
		int(res.Status),
	))
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == pqerror.UniqueViolation {
			return nil, model.ErrReferenceExists
		}
		return nil, fmt.Errorf("insert reservation: %w", err)
	}

	if err := insertReservationRooms(ctx, tx, result.ID, allocations); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	result.Rooms = allocations

	return result, nil
}

func allocateReservationRooms(ctx context.Context, tx *sql.Tx, res model.ReservationItem) ([]*model.ReservationRoomDetails, error) {
	if len(res.RoomIDs) > 0 {
		return lockRequestedRooms(ctx, tx, res)
	}

	candidates, err := lockAvailableRooms(ctx, tx, res)
	if err != nil {
		return nil, err
	}

	return allocateRooms(candidates, res.GuestsCount)
}

func lockRequestedRooms(ctx context.Context, tx *sql.Tx, res model.ReservationItem) ([]*model.ReservationRoomDetails, error) {
	roomIDs := slices.Compact(slices.Sorted(slices.Values(res.RoomIDs)))

	var known int
	if err := tx.QueryRowContext(
		ctx,
		countRequestedRoomsQuery,
		res.HotelID,
		pq.Array(roomIDs),
	).Scan(&known); err != nil {
		return nil, fmt.Errorf("check requested rooms: %w", err)
	}
	if known != len(roomIDs) {
		return nil, model.ErrRoomNotFound
	}

	rows, err := tx.QueryContext(ctx, lockRequestedRoomsQuery, res.HotelID, pq.Array(roomIDs), int(model.ReservationStatusCancelled), res.CheckInDate, res.CheckOutDate)
	if err != nil {
		return nil, fmt.Errorf("lock requested rooms: %w", err)
	}
	defer rows.Close()

	rooms := make([]candidateRoom, 0, len(roomIDs))
	for rows.Next() {
		var room candidateRoom
		if err := rows.Scan(&room.ID, &room.Label, &room.RoomTypeCaption, &room.RoomTypeCapacity); err != nil {
			return nil, fmt.Errorf("scan requested room: %w", err)
		}
		rooms = append(rooms, room)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate requested rooms: %w", err)
	}
	if len(rooms) != len(roomIDs) {
		return nil, model.ErrRoomUnavailable
	}

	slices.SortStableFunc(rooms, func(a, b candidateRoom) int {
		return b.RoomTypeCapacity - a.RoomTypeCapacity
	})

	return distributeGuests(rooms, res.GuestsCount)
}

func lockAvailableRooms(ctx context.Context, tx *sql.Tx, res model.ReservationItem) ([]candidateRoom, error) {
	rows, err := tx.QueryContext(ctx, lockAvailableRoomsQuery, res.HotelID, int(model.ReservationStatusCancelled), res.CheckInDate, res.CheckOutDate)
	if err != nil {
		return nil, fmt.Errorf("lock available rooms: %w", err)
	}
	defer rows.Close()

	rooms := make([]candidateRoom, 0)
	for rows.Next() {
		var room candidateRoom
		if err := rows.Scan(&room.ID, &room.Label, &room.RoomTypeCaption, &room.RoomTypeCapacity); err != nil {
			return nil, fmt.Errorf("scan candidate room: %w", err)
		}
		rooms = append(rooms, room)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate candidate rooms: %w", err)
	}

	return rooms, nil
}

func allocateRooms(candidates []candidateRoom, guestsCount int) ([]*model.ReservationRoomDetails, error) {
	single := -1
	for i := range candidates {
		if candidates[i].RoomTypeCapacity < guestsCount {
			continue
		}
		if single == -1 || candidates[i].RoomTypeCapacity < candidates[single].RoomTypeCapacity {
			single = i
		}
	}
	if single != -1 {
		return []*model.ReservationRoomDetails{newRoomAllocation(candidates[single], guestsCount)}, nil
	}

	sorted := slices.Clone(candidates)
	slices.SortStableFunc(sorted, func(a, b candidateRoom) int {
		return b.RoomTypeCapacity - a.RoomTypeCapacity
	})

	return distributeGuests(sorted, guestsCount)
}

func distributeGuests(rooms []candidateRoom, guestsCount int) ([]*model.ReservationRoomDetails, error) {
	remaining := guestsCount
	allocations := make([]*model.ReservationRoomDetails, 0, len(rooms))
	for _, room := range rooms {
		if remaining <= 0 {
			break
		}

		assigned := min(room.RoomTypeCapacity, remaining)
		allocations = append(allocations, newRoomAllocation(room, assigned))
		remaining -= assigned
	}

	if remaining > 0 {
		return nil, model.ErrInsufficientCapacity
	}

	return allocations, nil
}

func newRoomAllocation(room candidateRoom, guestsCount int) *model.ReservationRoomDetails {
	return &model.ReservationRoomDetails{
		RoomID:           room.ID,
		Label:            room.Label,
		RoomTypeCaption:  room.RoomTypeCaption,
		RoomTypeCapacity: room.RoomTypeCapacity,
		GuestsCount:      guestsCount,
	}
}

func insertReservationRooms(ctx context.Context, tx *sql.Tx, reservationID string, allocations []*model.ReservationRoomDetails) error {
	if len(allocations) == 0 {
		return nil
	}

	var (
		reservationIDs = make([]string, len(allocations))
		roomIDs        = make([]string, len(allocations))
		guestsCounts   = make([]int, len(allocations))
	)
	for i, allocation := range allocations {
		reservationIDs[i] = reservationID
		roomIDs[i] = allocation.RoomID
		guestsCounts[i] = allocation.GuestsCount
	}

	if _, err := tx.ExecContext(
		ctx,
		insertReservationRoomsQuery,
		pq.Array(reservationIDs),
		pq.Array(roomIDs),
		pq.Array(guestsCounts),
	); err != nil {
		return fmt.Errorf("insert reservation rooms: %w", err)
	}

	return nil
}

// GetReservationByReference retrieves a reservation by its booking reference from the database.
func (r *reservationRepository) GetReservationByReference(ctx context.Context, bookingRef string) (*model.ReservationItem, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	reservation, err := scanReservation(r.db.QueryRowContext(ctx, selectByReferenceQuery, bookingRef))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get reservation by reference: %w", err)
	}

	if err := r.loadRooms(ctx, []*model.ReservationItem{reservation}); err != nil {
		return nil, err
	}

	return reservation, nil
}

// GetReservationByID retrieves a reservation by its ID from the database.
func (r *reservationRepository) GetReservationByID(ctx context.Context, reservationID string) (*model.ReservationItem, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	reservation, err := scanReservation(r.db.QueryRowContext(ctx, selectByIDQuery, reservationID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get reservation by id: %w", err)
	}

	if err := r.loadRooms(ctx, []*model.ReservationItem{reservation}); err != nil {
		return nil, err
	}

	return reservation, nil
}

// UpdateReservationStatus updates the status of a reservation and returns the updated record.
func (r *reservationRepository) UpdateReservationStatus(ctx context.Context, reservationID string, status model.ReservationStatus) (*model.ReservationItem, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	reservation, err := scanReservation(r.db.QueryRowContext(ctx, updateReservationStatusQuery, reservationID, int(status)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrReservationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update reservation status: %w", err)
	}

	if err := r.loadRooms(ctx, []*model.ReservationItem{reservation}); err != nil {
		return nil, err
	}

	return reservation, nil
}

// GetAllReservationsByStatus retrieves all reservations with the specified status from the database.
func (r *reservationRepository) GetAllReservationsByStatus(ctx context.Context, status model.ReservationStatus) ([]*model.ReservationItem, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, selectByStatusQuery, int(status))
	if err != nil {
		return nil, fmt.Errorf("get reservations by status: %w", err)
	}
	defer rows.Close()

	reservations := make([]*model.ReservationItem, 0)
	for rows.Next() {
		reservation, err := scanReservation(rows)
		if err != nil {
			return nil, fmt.Errorf("scan reservation: %w", err)
		}
		reservations = append(reservations, reservation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate reservations: %w", err)
	}

	if err := r.loadRooms(ctx, reservations); err != nil {
		return nil, err
	}

	return reservations, nil
}

// DeleteAllReservations removes all reservations.
func (r *reservationRepository) DeleteAllReservations(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	if _, err := r.db.ExecContext(ctx, deleteAllReservationsQuery); err != nil {
		return fmt.Errorf("delete all reservations: %w", err)
	}

	return nil
}
