package server

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"

	appMock "hotel-booking-api/internal/app/server/mock"

	"github.com/stretchr/testify/mock"
)

func TestStartWorkers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		cancelBefore bool
		expect       func(worker *appMock.MockReservationWorker, ctx context.Context)
	}{
		{
			name: "starts the worker with the given context",
			expect: func(worker *appMock.MockReservationWorker, ctx context.Context) {
				worker.EXPECT().Start(ctx).Once()
			},
		},
		{
			name:         "passes a cancelled context through",
			cancelBefore: true,
			expect: func(worker *appMock.MockReservationWorker, ctx context.Context) {
				worker.EXPECT().
					Start(mock.MatchedBy(func(ctx context.Context) bool {
						return errors.Is(ctx.Err(), context.Canceled)
					})).
					Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if tt.cancelBefore {
				cancel()
			}

			worker := appMock.NewMockReservationWorker(t)

			server := &apiServer{workers: workers{reservationWorker: worker}}

			tt.expect(worker, ctx)

			server.startWorkers(ctx)
		})
	}
}

func TestStartWorkersWaitsForTheWorker(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})
		done := make(chan struct{})

		ctx := t.Context()

		worker := appMock.NewMockReservationWorker(t)
		worker.EXPECT().
			Start(ctx).
			Run(func(context.Context) { <-release }).
			Once()

		server := &apiServer{workers: workers{reservationWorker: worker}}

		go func() {
			defer close(done)
			server.startWorkers(ctx)
		}()

		synctest.Wait()

		select {
		case <-done:
			t.Fatal("startWorkers returned before the worker did")
		default:
		}

		close(release)

		synctest.Wait()

		select {
		case <-done:
		default:
			t.Fatal("startWorkers did not return after the worker did")
		}
	})
}
