package repository

import (
	"fmt"
)

// The statements of the reservation repository are built around two shared
// projections, so that the columns of a reservation and of its allocated rooms
// are spelled out exactly once.
const (
	// reservationSelect is the projection of a reservation joined with its
	// hotel. The table expression is a placeholder, because the very same
	// projection is used for plain reads as well as for the CTEs of the insert
	// and the update statements.
	reservationSelect = `
		SELECT res.id,
		       res.check_in_date::text, res.check_out_date::text,
		       res.guest_full_name, res.guest_email, res.guests_count,
		       res.reference, res.reservation_status,
		       res.hotel_id,
		       h.name, h.address
		FROM %s AS res
		INNER JOIN hotels AS h ON h.id = res.hotel_id
	`

	// reservationRoomsSelect loads the rooms that are allocated to the given
	// reservations.
	reservationRoomsSelect = `
		SELECT rr.reservation_id, rr.room_id, rr.guests_count,
		       rm.room_label, rt.caption, rt.capacity
		FROM reservation_rooms AS rr
		INNER JOIN rooms AS rm ON rm.id = rr.room_id
		INNER JOIN room_types AS rt ON rt.id = rm.room_type_id
		WHERE rr.reservation_id = ANY($1)
		ORDER BY rr.reservation_id, rm.room_label
	`

	// lockHotelQuery locks the hotel row so that concurrent bookings for the
	// same hotel serialize their room allocation.
	lockHotelQuery           = `SELECT id FROM hotels WHERE id = $1 FOR UPDATE`
	countRequestedRoomsQuery = `SELECT COUNT(*) FROM rooms WHERE hotel_id = $1 AND id = ANY($2)`

	// lockRequestedRoomsQuery locks the explicitly requested rooms that are
	// free for the requested dates. $3 is the cancelled status, $4 and $5 are
	// the check in and the check out date.
	lockRequestedRoomsQuery = `
		SELECT r.id, r.room_label, rt.caption, rt.capacity
		FROM rooms AS r
		INNER JOIN room_types AS rt ON rt.id = r.room_type_id
		WHERE r.hotel_id = $1
		  AND r.id = ANY($2)
		  AND r.is_available
		  AND NOT EXISTS (
		      SELECT 1
		      FROM reservation_rooms AS rr
		      INNER JOIN reservations AS res ON res.id = rr.reservation_id
		      WHERE rr.room_id = r.id
		        AND res.reservation_status <> $3
		        AND res.check_in_date < $5::date
		        AND res.check_out_date > $4::date
		  )
		ORDER BY r.id
		FOR UPDATE OF r
	`

	// lockAvailableRoomsQuery locks every free room of the hotel. $2 is the
	// cancelled status, $3 and $4 are the check in and the check out date.
	lockAvailableRoomsQuery = `
		SELECT r.id, r.room_label, rt.caption, rt.capacity
		FROM rooms AS r
		INNER JOIN room_types AS rt ON rt.id = r.room_type_id
		WHERE r.hotel_id = $1
		  AND r.is_available
		  AND NOT EXISTS (
		      SELECT 1
		      FROM reservation_rooms AS rr
		      INNER JOIN reservations AS res ON res.id = rr.reservation_id
		      WHERE rr.room_id = r.id
		        AND res.reservation_status <> $2
		        AND res.check_in_date < $4::date
		        AND res.check_out_date > $3::date
		  )
		ORDER BY r.id
		FOR UPDATE OF r
	`

	// insertReservationRoomsQuery links the allocated rooms to the reservation.
	insertReservationRoomsQuery = `
		INSERT INTO reservation_rooms (reservation_id, room_id, guests_count)
		SELECT * FROM unnest($1::text[], $2::text[], $3::int[])
	`

	// deleteAllReservationsQuery removes every reservation.
	deleteAllReservationsQuery = `DELETE FROM reservations`
)

// The statements below are the shared projections instantiated for a concrete
// table expression.
var (
	// insertReservationQuery inserts the booking header and returns it with the
	// generated identifier.
	insertReservationQuery = `
		WITH inserted AS (
			INSERT INTO reservations (
				id, reference, hotel_id, guest_full_name, guest_email,
				guests_count, check_in_date, check_out_date, reservation_status
			)
			VALUES (gen_random_uuid()::text, $1, $2, $3, $4, $5, $6::date, $7::date, $8)
			RETURNING *
		)
	` + fmt.Sprintf(reservationSelect, "inserted")

	selectByReferenceQuery = fmt.Sprintf(reservationSelect, "reservations") + " WHERE res.reference = $1"

	selectByIDQuery = fmt.Sprintf(reservationSelect, "reservations") + " WHERE res.id = $1"

	selectByStatusQuery = fmt.Sprintf(reservationSelect, "reservations") + `
		WHERE res.reservation_status = $1
		ORDER BY res.check_in_date, res.id
	`

	// updateReservationStatusQuery updates the status and returns the updated
	// reservation.
	updateReservationStatusQuery = `
		WITH updated AS (
			UPDATE reservations
			SET reservation_status = $2
			WHERE id = $1
			RETURNING *
		)
	` + fmt.Sprintf(reservationSelect, "updated")
)
