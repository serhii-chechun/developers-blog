package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	modelReservation "hotel-booking-api/internal/reservation/model"
	modelRoom "hotel-booking-api/internal/room/model"

	"hotel-booking-api/internal/helper/pagination"

	"github.com/lib/pq"
)

type (
	roomRepository struct {
		db           *sql.DB
		queryTimeout time.Duration
	}
)

// New creates a new instance of the room repository.
func New(db *sql.DB, queryTimeout time.Duration) *roomRepository {
	return &roomRepository{
		db:           db,
		queryTimeout: queryTimeout,
	}
}

// GetAvailableRooms retrieves all available rooms based on params provided.
func (r *roomRepository) GetAvailableRooms(ctx context.Context, p modelRoom.FindAvailableRoomsParams) (*modelRoom.RoomsPage, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	rows, err := r.db.QueryContext(
		ctx,
		selectAvailableRoomsQuery,
		p.GuestsCount,
		p.HotelID,
		p.AfterID,
		int(modelReservation.ReservationStatusCancelled),
		p.CheckInDate,
		p.CheckOutDate,
		p.PageSize+1,
	)
	if err != nil {
		return nil, fmt.Errorf("find available rooms: %w", err)
	}
	defer rows.Close()

	rooms := make([]*modelRoom.RoomItem, 0, p.PageSize)
	for rows.Next() {
		var (
			room      modelRoom.RoomItem
			roomType  modelRoom.RoomType
			hotelInfo modelRoom.RoomItemHotelDetails
		)
		if err := rows.Scan(
			&room.ID,
			&room.HotelID,
			&room.Label,
			&room.IsAvailable,
			&roomType.ID,
			&roomType.Caption,
			&roomType.Capacity,
			&hotelInfo.Name,
			&hotelInfo.Address,
		); err != nil {
			return nil, fmt.Errorf("scan room: %w", err)
		}

		room.TypeID = roomType.ID
		room.RoomType = &roomType
		room.RoomItemHotelDetails = &hotelInfo
		rooms = append(rooms, &room)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rooms: %w", err)
	}

	page := &modelRoom.RoomsPage{
		Rooms:    rooms,
		PageSize: p.PageSize,
	}

	if len(rooms) > p.PageSize {
		page.Rooms = rooms[:p.PageSize]

		nextPage, err := pagination.Encode(page.Rooms[len(page.Rooms)-1].ID)
		if err != nil {
			return nil, fmt.Errorf("encode page token: %w", err)
		}
		page.NextPage = nextPage
	}

	return page, nil
}

// GetRoomTypes retrieves all available room types.
func (r *roomRepository) GetRoomTypes(ctx context.Context) ([]*modelRoom.RoomType, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, selectRoomTypesQuery)
	if err != nil {
		return nil, fmt.Errorf("get room types: %w", err)
	}
	defer rows.Close()

	roomTypes := make([]*modelRoom.RoomType, 0)
	for rows.Next() {
		var roomType modelRoom.RoomType
		if err := rows.Scan(&roomType.ID, &roomType.Caption, &roomType.Capacity); err != nil {
			return nil, fmt.Errorf("scan room type: %w", err)
		}
		roomTypes = append(roomTypes, &roomType)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate room types: %w", err)
	}

	return roomTypes, nil
}

// PutRooms inserts the provided rooms in a single statement.
func (r *roomRepository) PutRooms(ctx context.Context, rooms []*modelRoom.RoomItem) error {
	if len(rooms) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

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

	if _, err := r.db.ExecContext(
		ctx,
		insertRoomsQuery,
		pq.Array(ids),
		pq.Array(hotelIDs),
		pq.Array(typeIDs),
		pq.Array(labels),
		pq.Array(availability),
	); err != nil {
		return fmt.Errorf("insert rooms: %w", err)
	}

	return nil
}

// DeleteAllRooms removes all rooms together with their dependent records.
func (r *roomRepository) DeleteAllRooms(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	if _, err := r.db.ExecContext(ctx, deleteAllRoomsQuery); err != nil {
		return fmt.Errorf("delete all rooms: %w", err)
	}

	return nil
}
