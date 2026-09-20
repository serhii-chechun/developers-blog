package server

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"

	storagePostgres "hotel-booking-api/internal/app/storage/postgres"
	handlerTesting "hotel-booking-api/internal/app/testing/handler"
	serviceTesting "hotel-booking-api/internal/app/testing/service"

	handlerHotel "hotel-booking-api/internal/hotel/handler"
	handlerReservation "hotel-booking-api/internal/reservation/handler"
	handlerRoom "hotel-booking-api/internal/room/handler"

	serviceHotel "hotel-booking-api/internal/hotel/service"
	serviceReservation "hotel-booking-api/internal/reservation/service"
	serviceRoom "hotel-booking-api/internal/room/service"

	repositoryHotel "hotel-booking-api/internal/hotel/repository"
	repositoryReservation "hotel-booking-api/internal/reservation/repository"
	repositoryRoom "hotel-booking-api/internal/room/repository"

	workerReservation "hotel-booking-api/internal/reservation/worker"

	"github.com/gin-gonic/gin"
)

type (
	handlers struct {
		hotelHandler       hotelHandler
		roomHandler        roomHandler
		reservationHandler reservationHandler
		testingHandler     testingHandler
	}

	hotelHandler interface {
		GetAll(c *gin.Context)
	}

	roomHandler interface {
		GetAll(c *gin.Context)
	}

	reservationHandler interface {
		Create(c *gin.Context)
		Get(c *gin.Context)
	}

	testingHandler interface {
		Seed(c *gin.Context)
		Reset(c *gin.Context)
	}

	workers struct {
		reservationWorker reservationWorker
	}

	reservationWorker interface {
		Start(ctx context.Context)
	}

	storage interface {
		Connect() (*sql.DB, error)
		Close() error
	}
)

func (s *apiServer) init() error {
	s.log = slog.New(slog.NewJSONHandler(os.Stdout, nil))

	s.storage = storagePostgres.New(
		s.log,
		storagePostgres.Params{
			Username:          s.config.PostgresUser,
			Password:          s.config.PostgresPassword,
			Database:          s.config.PostgresDB,
			Host:              s.config.PostgresHost,
			Port:              s.config.PostgresPort,
			ConnectMaxRetries: s.config.DatabaseConnectMaxRetries,
			ConnectRetryDelay: s.config.DatabaseConnectRetryDelay,
			ConnMaxIdleTime:   s.config.DatabaseConnMaxIdleTime,
			ConnMaxLifetime:   s.config.DatabaseConnMaxLifetime,
			MaxIdleConns:      s.config.DatabaseMaxIdleConns,
			MaxOpenConns:      s.config.DatabaseMaxOpenConns,
		})

	db, err := s.storage.Connect()
	if err != nil {
		return fmt.Errorf("storage connection: %w", err)
	}

	reservationService := serviceReservation.New(
		repositoryReservation.New(db, s.config.DatabaseQueryTimeout),
	)

	hotelService := serviceHotel.New(
		repositoryHotel.New(db, s.config.DatabaseQueryTimeout),
	)

	roomService := serviceRoom.New(
		repositoryRoom.New(db, s.config.DatabaseQueryTimeout),
	)

	s.workers.reservationWorker = workerReservation.New(
		reservationService,
		s.config.WorkerInterval,
		s.log,
	)

	s.handlerMux = s.registerRoutes(
		gin.Default(),
		&handlers{
			hotelHandler: handlerHotel.New(
				hotelService,
			),
			roomHandler: handlerRoom.New(
				roomService,
			),
			reservationHandler: handlerReservation.New(
				reservationService,
			),
			testingHandler: handlerTesting.New(
				serviceTesting.New(
					hotelService,
					roomService,
					reservationService,
				),
			),
		})

	return nil
}

func (s *apiServer) registerRoutes(r *gin.Engine, h *handlers) *gin.Engine {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})

	v1 := r.Group("/v1")
	{
		hotels := v1.Group("/hotels")
		{
			hotels.GET("", h.hotelHandler.GetAll)
		}

		rooms := v1.Group("/rooms")
		{
			rooms.GET("", h.roomHandler.GetAll)
		}

		reservations := v1.Group("/reservations")
		{
			reservations.GET("/:booking_ref", h.reservationHandler.Get)
			reservations.POST("", h.reservationHandler.Create)
		}

		testing := v1.Group("/testing")
		{
			testing.PUT("", h.testingHandler.Seed)
			testing.DELETE("", h.testingHandler.Reset)
		}
	}
	return r
}
