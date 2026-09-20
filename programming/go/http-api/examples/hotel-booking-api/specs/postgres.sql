CREATE TABLE IF NOT EXISTS hotels (
    id   VARCHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    address VARCHAR(255) NOT NULL,
    phone VARCHAR(50) NOT NULL
);

CREATE TABLE IF NOT EXISTS room_types (
    id   INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    caption VARCHAR(255) NOT NULL UNIQUE,
    capacity INT NOT NULL
);

INSERT INTO room_types (caption, capacity) VALUES
    ('single', 1),
    ('double', 2),
    ('deluxe', 5)
ON CONFLICT (caption) DO NOTHING;

CREATE TABLE IF NOT EXISTS rooms (
    id   VARCHAR(36) PRIMARY KEY,
    hotel_id VARCHAR(36) NOT NULL,
    room_type_id INT NOT NULL,
    room_label VARCHAR(255) NOT NULL,
    is_available BOOLEAN NOT NULL DEFAULT TRUE,
    FOREIGN KEY (hotel_id) REFERENCES hotels (id) ON DELETE CASCADE,
    FOREIGN KEY (room_type_id) REFERENCES room_types (id) ON DELETE CASCADE,
    UNIQUE (hotel_id, room_label)
);
CREATE INDEX idx_room_type_id ON rooms (room_type_id);
CREATE INDEX idx_room_is_available ON rooms (is_available);

CREATE TABLE IF NOT EXISTS reservations (
    id   VARCHAR(36) PRIMARY KEY,
    reference VARCHAR(10) NOT NULL,
    hotel_id VARCHAR(36) NOT NULL,
    guest_full_name VARCHAR(255) NOT NULL,
    guest_email VARCHAR(255) NOT NULL,
    guests_count INT NOT NULL,
    check_in_date DATE NOT NULL,
    check_out_date DATE NOT NULL,
    reservation_status INT NOT NULL,
    FOREIGN KEY (hotel_id) REFERENCES hotels (id) ON DELETE CASCADE,
    UNIQUE (reference)
);
CREATE INDEX idx_reservation_hotel_id ON reservations (hotel_id);
CREATE INDEX idx_guest_full_name ON reservations (guest_full_name);
CREATE INDEX idx_guest_email ON reservations (guest_email);
CREATE INDEX idx_check_in_date ON reservations (check_in_date);
CREATE INDEX idx_check_out_date ON reservations (check_out_date);
CREATE INDEX idx_reservation_status ON reservations (reservation_status);

CREATE TABLE IF NOT EXISTS reservation_rooms (
    reservation_id VARCHAR(36) NOT NULL,
    room_id VARCHAR(36) NOT NULL,
    guests_count INT NOT NULL,
    PRIMARY KEY (reservation_id, room_id),
    FOREIGN KEY (reservation_id) REFERENCES reservations (id) ON DELETE CASCADE,
    FOREIGN KEY (room_id) REFERENCES rooms (id) ON DELETE CASCADE
);
CREATE INDEX idx_reservation_rooms_room_id ON reservation_rooms (room_id);
CREATE INDEX idx_reservation_rooms_reservation_id ON reservation_rooms (reservation_id);
