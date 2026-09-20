package postgres

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/lib/pq"
)

type (
	storage struct {
		db               *sql.DB
		log              *slog.Logger
		connectionString string

		*connectionParams
	}

	// Params holds the configuration required to establish a connection to the Postgres database.
	Params struct {
		Username string
		Password string
		Database string
		Host     string
		Port     int

		connectionParams
	}

	connectionParams struct {
		ConnectMaxRetries int
		ConnectRetryDelay time.Duration

		ConnMaxIdleTime time.Duration
		ConnMaxLifetime time.Duration

		MaxIdleConns int
		MaxOpenConns int
	}
)

// New creates a new instance of Postgres.
func New(l *slog.Logger, p Params) *storage {
	return &storage{
		log: l,
		connectionString: fmt.Sprintf(
			"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			p.Host,
			p.Port,
			p.Username,
			p.Password,
			p.Database,
		),
		connectionParams: &p.connectionParams,
	}
}

// Connect establishes a connection to the Postgres database.
func (s *storage) Connect() (*sql.DB, error) {
	db, err := s.connectWithRetry()
	if err != nil {
		return nil, err
	}
	s.db = db
	return db, nil
}

// Close closes the connection to the Postgres database.
func (s *storage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *storage) connectWithRetry() (*sql.DB, error) {
	var (
		db   *sql.DB
		err  error
		done bool = false
	)

	attempt := 1
	for !done {
		if attempt > s.connectionParams.ConnectMaxRetries {
			return nil, fmt.Errorf("maximum number of the DB connection attempts exceeded")
		} else if attempt > 1 {
			time.Sleep(s.connectionParams.ConnectRetryDelay)
		}

		db, err = sql.Open(`postgres`, s.connectionString)
		if err != nil {
			return nil, err
		}

		if err := db.Ping(); err != nil {
			s.log.Warn("database connection is down: " + err.Error())
			attempt++
			continue
		}
		done = true
	}

	if s.connectionParams.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(s.connectionParams.ConnMaxIdleTime)
	}

	if s.connectionParams.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(s.connectionParams.ConnMaxLifetime)
	}

	if s.connectionParams.MaxIdleConns > 0 {
		db.SetMaxIdleConns(s.connectionParams.MaxIdleConns)
	}

	if s.connectionParams.MaxOpenConns > 0 {
		db.SetMaxOpenConns(s.connectionParams.MaxOpenConns)
	}

	return db, nil
}
