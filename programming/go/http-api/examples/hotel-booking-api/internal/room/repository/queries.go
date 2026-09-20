package repository

const (
	// selectAvailableRoomsQuery returns one page of rooms that are free for the
	// requested dates and whose hotel has enough capacity for the party. It
	// always asks for one row more than the page size, so that the repository
	// can tell whether another page follows.
	selectAvailableRoomsQuery = `
		WITH free_rooms AS (
		    SELECT r.id, r.hotel_id, r.room_label, r.is_available,
		           rt.id AS room_type_id, rt.caption, rt.capacity
		    FROM rooms AS r
		    INNER JOIN room_types AS rt ON rt.id = r.room_type_id
		    WHERE r.is_available
		      AND NOT EXISTS (
		          SELECT 1
		          FROM reservation_rooms AS rr
		          INNER JOIN reservations AS res ON res.id = rr.reservation_id
		          WHERE rr.room_id = r.id
		            AND res.reservation_status <> $4
		            AND res.check_in_date < $6::date
		            AND res.check_out_date > $5::date
		      )
		),
		hotel_free_capacity AS (
		    SELECT hotel_id, SUM(capacity) AS total_capacity
		    FROM free_rooms
		    GROUP BY hotel_id
		)
		SELECT fr.id, fr.hotel_id, fr.room_label, fr.is_available,
		       fr.room_type_id, fr.caption, fr.capacity,
		       h.name, h.address
		FROM free_rooms AS fr
		INNER JOIN hotels AS h ON h.id = fr.hotel_id
		INNER JOIN hotel_free_capacity AS hfc ON hfc.hotel_id = fr.hotel_id
		WHERE hfc.total_capacity >= $1
		  AND ($2 = '' OR fr.hotel_id = $2)
		  AND ($3 = '' OR fr.id > $3)
		ORDER BY fr.id
		LIMIT $7
	`

	// selectRoomTypesQuery returns every room type ordered by id.
	selectRoomTypesQuery = `
		SELECT id, caption, capacity
		FROM room_types
		ORDER BY id
	`

	// insertRoomsQuery inserts every room with a single statement.
	insertRoomsQuery = `
		INSERT INTO rooms (id, hotel_id, room_type_id, room_label, is_available)
		SELECT * FROM unnest($1::text[], $2::text[], $3::int[], $4::text[], $5::bool[])
	`

	// deleteAllRoomsQuery removes every room together with its dependent records.
	deleteAllRoomsQuery = `DELETE FROM rooms`
)
