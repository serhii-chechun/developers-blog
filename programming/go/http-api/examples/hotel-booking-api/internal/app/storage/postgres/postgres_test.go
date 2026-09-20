package postgres

import (
	"bytes"
	"database/sql"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		params Params
		want   string
	}{
		{
			name: "builds the connection string from the params",
			params: Params{
				Username: "booking",
				Password: "secret",
				Database: "hotels",
				Host:     "db.internal",
				Port:     5432,
				connectionParams: connectionParams{
					ConnectMaxRetries: 3,
					ConnectRetryDelay: time.Second,
					ConnMaxIdleTime:   time.Minute,
					ConnMaxLifetime:   5 * time.Minute,
					MaxIdleConns:      2,
					MaxOpenConns:      10,
				},
			},
			want: "host=db.internal port=5432 user=booking password=secret dbname=hotels sslmode=disable",
		},
		{
			name:   "builds the connection string from empty params",
			params: Params{},
			want:   "host= port=0 user= password= dbname= sslmode=disable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

			storage := New(logger, tt.params)

			assert.Equal(t, tt.want, storage.connectionString)
			assert.Equal(t, tt.params.connectionParams, *storage.connectionParams)
			assert.Same(t, logger, storage.log)
			assert.Nil(t, storage.db)
		})
	}
}

func TestConnect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		maxRetries      int
		retryDelay      time.Duration
		wantAttempts    int
		wantMinDuration time.Duration
	}{
		{
			name:         "gives up without trying when no retry is allowed",
			maxRetries:   0,
			wantAttempts: 0,
		},
		{
			name:         "gives up after a single attempt",
			maxRetries:   1,
			wantAttempts: 1,
		},
		{
			name:         "gives up after exhausting the retries",
			maxRetries:   3,
			wantAttempts: 3,
		},
		{
			name:            "waits between the attempts",
			maxRetries:      3,
			retryDelay:      20 * time.Millisecond,
			wantAttempts:    3,
			wantMinDuration: 40 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var logs bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&logs, nil))

			storage := New(logger, Params{
				Username: "booking",
				Password: "secret",
				Database: "hotels",
				Host:     "127.0.0.1",
				Port:     1,
				connectionParams: connectionParams{
					ConnectMaxRetries: tt.maxRetries,
					ConnectRetryDelay: tt.retryDelay,
				},
			})

			started := time.Now()
			db, err := storage.Connect()
			elapsed := time.Since(started)

			require.ErrorContains(t, err, "maximum number of the DB connection attempts exceeded")
			assert.Nil(t, db)
			assert.Nil(t, storage.db)
			assert.Equal(t, tt.wantAttempts, strings.Count(logs.String(), "database connection is down"))
			assert.GreaterOrEqual(t, elapsed, tt.wantMinDuration)
		})
	}
}

func TestClose(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		withDB bool
	}{
		{
			name:   "does nothing when there is no connection",
			withDB: false,
		},
		{
			name:   "closes the connection",
			withDB: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))
			storage := New(logger, Params{Host: "127.0.0.1", Port: 1})

			var (
				db      *sql.DB
				sqlMock sqlmock.Sqlmock
			)

			if tt.withDB {
				var err error
				db, sqlMock, err = sqlmock.New()
				require.NoError(t, err)
				require.NoError(t, db.Ping())
				sqlMock.ExpectClose()
				storage.db = db
			}

			require.NoError(t, storage.Close())

			if !tt.withDB {
				return
			}

			require.NoError(t, sqlMock.ExpectationsWereMet())
			require.ErrorContains(t, db.Ping(), "database is closed")
		})
	}
}
