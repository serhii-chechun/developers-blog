package repository

const (
	// selectHotelsQuery returns one page of hotels whose name matches the
	// pattern. It always asks for one row more than the page size, so that the
	// repository can tell whether another page follows.
	selectHotelsQuery = `
		SELECT id, name, address, phone
		FROM hotels
		WHERE name ILIKE '%' || $1 || '%'
		  AND ($2 = '' OR id > $2)
		ORDER BY id
		LIMIT $3
	`

	// insertHotelsQuery inserts every hotel with a single statement.
	insertHotelsQuery = `
		INSERT INTO hotels (id, name, address, phone)
		SELECT * FROM unnest($1::text[], $2::text[], $3::text[], $4::text[])
	`

	// deleteAllHotelsQuery removes every hotel together with its dependent records.
	deleteAllHotelsQuery = `DELETE FROM hotels`
)
