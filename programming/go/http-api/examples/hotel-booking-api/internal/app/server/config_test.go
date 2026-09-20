package server

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var configEnvNames = []string{
	"LISTEN_ADDRESS",
	"SHUTDOWN_TIMEOUT",
	"WORKER_INTERVAL",
	"POSTGRES_USER",
	"POSTGRES_PASSWORD",
	"POSTGRES_DB",
	"POSTGRES_HOST",
	"POSTGRES_PORT",
	"DB_CONNECT_MAX_RETRIES",
	"DB_CONNECT_RETRY_DELAY",
	"DB_QUERY_TIMEOUT",
	"DB_CONN_MAX_IDLE_TIME",
	"DB_CONN_MAX_LIFETIME",
	"DB_MAX_IDLE_CONNS",
	"DB_MAX_OPEN_CONNS",
}

func isolateConfigEnv(t *testing.T) {
	t.Helper()

	for _, name := range configEnvNames {
		value, ok := os.LookupEnv(name)
		if !ok {
			continue
		}

		require.NoError(t, os.Unsetenv(name))
		t.Cleanup(func() { _ = os.Setenv(name, value) })
	}
}

func TestProcessConfig(t *testing.T) {
	isolateConfigEnv(t)

	tests := []struct {
		name       string
		env        map[string]string
		want       config
		wantAnyErr bool
	}{
		{
			name: "applies the defaults when nothing is set",
			env:  map[string]string{},
			want: config{
				ListenAddress:             ":8080",
				ShutdownTimeout:           10 * time.Second,
				WorkerInterval:            10 * time.Second,
				PostgresUser:              "booking_api",
				PostgresPassword:          "booking_api",
				PostgresDB:                "booking_api",
				PostgresHost:              "localhost",
				PostgresPort:              5432,
				DatabaseConnectMaxRetries: 5,
				DatabaseConnectRetryDelay: 5 * time.Second,
				DatabaseQueryTimeout:      5 * time.Second,
				DatabaseConnMaxIdleTime:   5 * time.Minute,
				DatabaseConnMaxLifetime:   30 * time.Minute,
				DatabaseMaxIdleConns:      10,
				DatabaseMaxOpenConns:      100,
			},
		},
		{
			name: "reads every value from the environment",
			env: map[string]string{
				"LISTEN_ADDRESS":         ":9090",
				"SHUTDOWN_TIMEOUT":       "1s",
				"WORKER_INTERVAL":        "2s",
				"POSTGRES_USER":          "user",
				"POSTGRES_PASSWORD":      "secret",
				"POSTGRES_DB":            "db",
				"POSTGRES_HOST":          "db.internal",
				"POSTGRES_PORT":          "6543",
				"DB_CONNECT_MAX_RETRIES": "1",
				"DB_CONNECT_RETRY_DELAY": "250ms",
				"DB_QUERY_TIMEOUT":       "3s",
				"DB_CONN_MAX_IDLE_TIME":  "2m",
				"DB_CONN_MAX_LIFETIME":   "15m",
				"DB_MAX_IDLE_CONNS":      "3",
				"DB_MAX_OPEN_CONNS":      "7",
			},
			want: config{
				ListenAddress:             ":9090",
				ShutdownTimeout:           time.Second,
				WorkerInterval:            2 * time.Second,
				PostgresUser:              "user",
				PostgresPassword:          "secret",
				PostgresDB:                "db",
				PostgresHost:              "db.internal",
				PostgresPort:              6543,
				DatabaseConnectMaxRetries: 1,
				DatabaseConnectRetryDelay: 250 * time.Millisecond,
				DatabaseQueryTimeout:      3 * time.Second,
				DatabaseConnMaxIdleTime:   2 * time.Minute,
				DatabaseConnMaxLifetime:   15 * time.Minute,
				DatabaseMaxIdleConns:      3,
				DatabaseMaxOpenConns:      7,
			},
		},
		{
			name:       "fails on an invalid duration",
			env:        map[string]string{"SHUTDOWN_TIMEOUT": "soon"},
			wantAnyErr: true,
		},
		{
			name:       "fails on an invalid port",
			env:        map[string]string{"POSTGRES_PORT": "http"},
			wantAnyErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for name, value := range tt.env {
				t.Setenv(name, value)
			}

			server := New()

			err := server.processConfig(context.Background())

			if tt.wantAnyErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, server.config)
		})
	}
}
