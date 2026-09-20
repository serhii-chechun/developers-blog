package server

import (
	"context"
	"time"

	"github.com/sethvargo/go-envconfig"
)

type (
	config struct {
		ListenAddress   string        `env:"LISTEN_ADDRESS,default=:8080"`
		ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT,default=10s"`

		WorkerInterval time.Duration `env:"WORKER_INTERVAL,default=10s"`

		PostgresUser     string `env:"POSTGRES_USER,default=booking_api"`
		PostgresPassword string `env:"POSTGRES_PASSWORD,default=booking_api"`
		PostgresDB       string `env:"POSTGRES_DB,default=booking_api"`
		PostgresHost     string `env:"POSTGRES_HOST,default=localhost"`
		PostgresPort     int    `env:"POSTGRES_PORT,default=5432"`

		DatabaseConnectMaxRetries int           `env:"DB_CONNECT_MAX_RETRIES,default=5"`
		DatabaseConnectRetryDelay time.Duration `env:"DB_CONNECT_RETRY_DELAY,default=5s"`

		DatabaseQueryTimeout time.Duration `env:"DB_QUERY_TIMEOUT,default=5s"`

		DatabaseConnMaxIdleTime time.Duration `env:"DB_CONN_MAX_IDLE_TIME,default=5m"`
		DatabaseConnMaxLifetime time.Duration `env:"DB_CONN_MAX_LIFETIME,default=30m"`
		DatabaseMaxIdleConns    int           `env:"DB_MAX_IDLE_CONNS,default=10"`
		DatabaseMaxOpenConns    int           `env:"DB_MAX_OPEN_CONNS,default=100"`
	}
)

func (s *apiServer) processConfig(ctx context.Context) error {
	if err := envconfig.Process(ctx, &s.config); err != nil {
		return err
	}
	return nil
}
