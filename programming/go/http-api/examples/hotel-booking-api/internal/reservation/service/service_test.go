package service

import (
	"context"
	"errors"
	"testing"

	"hotel-booking-api/internal/reservation/model"
	"hotel-booking-api/internal/reservation/service/mock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testifymock "github.com/stretchr/testify/mock"
)

var errDefault = errors.New("error default")

func newTestService(t *testing.T, expectations func(repo *mock.MockReservationRepository)) *reservationService {
	t.Helper()

	repo := mock.NewMockReservationRepository(t)
	expectations(repo)

	return New(repo)
}

func pendingBookingWithFreshReference(booking model.ReservationItem) any {
	return testifymock.MatchedBy(func(r model.ReservationItem) bool {
		return r.Status == model.ReservationStatusPending &&
			r.Reference != "" &&
			r.Reference != booking.Reference &&
			len(r.Reference) == referenceLength &&
			r.HotelID == booking.HotelID &&
			r.GuestFullName == booking.GuestFullName &&
			r.CheckInDate == booking.CheckInDate
	})
}

func TestReservationService_CreateReservation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	booking := model.ReservationItem{
		Reference:     "CLIENT-REF",
		Status:        model.ReservationStatusCompleted,
		HotelID:       "h1",
		GuestFullName: "Jane Doe",
		GuestEmail:    "jane@example.com",
		GuestsCount:   2,
		CheckInDate:   "2026-10-01",
		CheckOutDate:  "2026-10-03",
	}

	stored := &model.ReservationItem{
		ID:            "res-1",
		Reference:     "GENERATED1",
		Status:        model.ReservationStatusPending,
		HotelID:       "h1",
		GuestFullName: "Jane Doe",
		GuestEmail:    "jane@example.com",
		GuestsCount:   2,
		CheckInDate:   "2026-10-01",
		CheckOutDate:  "2026-10-03",
	}

	taken := &model.ReservationItem{ID: "res-taken", Reference: "TAKEN"}

	tests := []struct {
		name         string
		expectations func(repo *mock.MockReservationRepository)
		want         *model.ReservationItem
		wantErr      error
	}{
		{
			name: "creates a pending reservation with a fresh reference",
			expectations: func(repo *mock.MockReservationRepository) {
				repo.EXPECT().
					GetReservationByReference(ctx, testifymock.Anything).
					Return(nil, nil).
					Once()
				repo.EXPECT().
					PutReservation(ctx, pendingBookingWithFreshReference(booking)).
					Return(stored, nil).
					Once()
			},
			want: stored,
		},
		{
			name: "retries while the generated reference is taken",
			expectations: func(repo *mock.MockReservationRepository) {
				repo.EXPECT().
					GetReservationByReference(ctx, testifymock.Anything).
					Return(taken, nil).
					Times(2)
				repo.EXPECT().
					GetReservationByReference(ctx, testifymock.Anything).
					Return(nil, nil).
					Once()
				repo.EXPECT().
					PutReservation(ctx, pendingBookingWithFreshReference(booking)).
					Return(stored, nil).
					Once()
			},
			want: stored,
		},
		{
			name: "gives up when every attempt collides",
			expectations: func(repo *mock.MockReservationRepository) {
				repo.EXPECT().
					GetReservationByReference(ctx, testifymock.Anything).
					Return(taken, nil).
					Times(maxReferenceAttempts)
			},
			wantErr: model.ErrReferenceExists,
		},
		{
			name: "propagates the lookup error",
			expectations: func(repo *mock.MockReservationRepository) {
				repo.EXPECT().
					GetReservationByReference(ctx, testifymock.Anything).
					Return(nil, errDefault).
					Once()
			},
			wantErr: errDefault,
		},
		{
			name: "propagates the insert error",
			expectations: func(repo *mock.MockReservationRepository) {
				repo.EXPECT().
					GetReservationByReference(ctx, testifymock.Anything).
					Return(nil, nil).
					Once()
				repo.EXPECT().
					PutReservation(ctx, testifymock.Anything).
					Return(nil, errDefault).
					Once()
			},
			wantErr: errDefault,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := newTestService(t, tt.expectations).CreateReservation(ctx, booking)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, got)
				return
			}

			require.NoError(t, err)
			assert.Same(t, tt.want, got)
		})
	}
}

func TestReservationService_FindReservationByReference(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	reservation := &model.ReservationItem{ID: "res-1", Reference: "REF-1", Status: model.ReservationStatusPending}

	tests := []struct {
		name         string
		bookingRef   string
		expectations func(repo *mock.MockReservationRepository)
		want         *model.ReservationItem
		wantErr      error
	}{
		{
			name:       "returns the reservation of the repository",
			bookingRef: "REF-1",
			expectations: func(repo *mock.MockReservationRepository) {
				repo.EXPECT().GetReservationByReference(ctx, "REF-1").Return(reservation, nil).Once()
			},
			want: reservation,
		},
		{
			name:       "returns nil for an unknown reference",
			bookingRef: "NOPE",
			expectations: func(repo *mock.MockReservationRepository) {
				repo.EXPECT().GetReservationByReference(ctx, "NOPE").Return(nil, nil).Once()
			},
		},
		{
			name:       "propagates the repository error",
			bookingRef: "REF-1",
			expectations: func(repo *mock.MockReservationRepository) {
				repo.EXPECT().GetReservationByReference(ctx, "REF-1").Return(nil, errDefault).Once()
			},
			wantErr: errDefault,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := newTestService(t, tt.expectations).FindReservationByReference(ctx, tt.bookingRef)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, got)
				return
			}

			require.NoError(t, err)
			assert.Same(t, tt.want, got)
		})
	}
}

func TestReservationService_FindReservationByStatus(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	reservations := []*model.ReservationItem{
		{ID: "res-1", Reference: "REF-1", Status: model.ReservationStatusPending},
		{ID: "res-2", Reference: "REF-2", Status: model.ReservationStatusPending},
	}

	tests := []struct {
		name         string
		status       model.ReservationStatus
		expectations func(repo *mock.MockReservationRepository)
		want         []*model.ReservationItem
		wantErr      error
	}{
		{
			name:   "returns the reservations of the repository",
			status: model.ReservationStatusPending,
			expectations: func(repo *mock.MockReservationRepository) {
				repo.EXPECT().
					GetAllReservationsByStatus(ctx, model.ReservationStatusPending).
					Return(reservations, nil).
					Once()
			},
			want: reservations,
		},
		{
			name:   "returns an empty list as is",
			status: model.ReservationStatusCancelled,
			expectations: func(repo *mock.MockReservationRepository) {
				repo.EXPECT().
					GetAllReservationsByStatus(ctx, model.ReservationStatusCancelled).
					Return([]*model.ReservationItem{}, nil).
					Once()
			},
			want: []*model.ReservationItem{},
		},
		{
			name:   "propagates the repository error",
			status: model.ReservationStatusPending,
			expectations: func(repo *mock.MockReservationRepository) {
				repo.EXPECT().
					GetAllReservationsByStatus(ctx, model.ReservationStatusPending).
					Return(nil, errDefault).
					Once()
			},
			wantErr: errDefault,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := newTestService(t, tt.expectations).FindReservationByStatus(ctx, tt.status)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, got)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReservationService_ConfirmReservation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	confirmed := &model.ReservationItem{ID: "res-1", Reference: "REF-1", Status: model.ReservationStatusConfirmed}

	tests := []struct {
		name         string
		expectations func(repo *mock.MockReservationRepository)
		want         *model.ReservationItem
		wantErr      error
	}{
		{
			name: "confirms the reservation through the repository",
			expectations: func(repo *mock.MockReservationRepository) {
				repo.EXPECT().
					UpdateReservationStatus(ctx, "res-1", model.ReservationStatusConfirmed).
					Return(confirmed, nil).
					Once()
			},
			want: confirmed,
		},
		{
			name: "propagates the repository error",
			expectations: func(repo *mock.MockReservationRepository) {
				repo.EXPECT().
					UpdateReservationStatus(ctx, "res-1", model.ReservationStatusConfirmed).
					Return(nil, model.ErrReservationNotFound).
					Once()
			},
			wantErr: model.ErrReservationNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := newTestService(t, tt.expectations).ConfirmReservation(ctx, "res-1")

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, got)
				return
			}

			require.NoError(t, err)
			assert.Same(t, tt.want, got)
		})
	}
}

func TestReservationService_RemoveAllReservations(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	tests := []struct {
		name         string
		expectations func(repo *mock.MockReservationRepository)
		wantErr      error
	}{
		{
			name: "removes everything through the repository",
			expectations: func(repo *mock.MockReservationRepository) {
				repo.EXPECT().DeleteAllReservations(ctx).Return(nil).Once()
			},
		},
		{
			name: "propagates the repository error",
			expectations: func(repo *mock.MockReservationRepository) {
				repo.EXPECT().DeleteAllReservations(ctx).Return(errDefault).Once()
			},
			wantErr: errDefault,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := newTestService(t, tt.expectations).RemoveAllReservations(ctx)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
