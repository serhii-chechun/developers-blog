package server

import (
	"context"
)

func (s *apiServer) startWorkers(ctx context.Context) {
	s.workers.reservationWorker.Start(ctx)
}
