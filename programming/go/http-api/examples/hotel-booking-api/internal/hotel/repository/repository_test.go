package repository

import (
	"context"
	"database/sql/driver"
	"errors"
	"regexp"
	"testing"
	"time"

	"hotel-booking-api/internal/helper/pagination"
	"hotel-booking-api/internal/hotel/model"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testQueryTimeout = time.Second
)

var (
	errDefault = errors.New("error default")
)

func newTestRepository(t *testing.T, expectations func(mock sqlmock.Sqlmock)) *hotelRepository {
	t.Helper()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	t.Cleanup(func() { _ = db.Close() })
	t.Cleanup(func() { assert.NoError(t, mock.ExpectationsWereMet()) })

	expectations(mock)

	return New(db, testQueryTimeout)
}

func hotelRows(hotels ...*model.HotelItem) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{"id", "name", "address", "phone"})
	for _, h := range hotels {
		rows.AddRow(h.ID, h.Name, h.Address, h.Phone)
	}
	return rows
}

func hotelInsertArgs(hotels ...*model.HotelItem) []driver.Value {
	var (
		ids       = make([]string, len(hotels))
		names     = make([]string, len(hotels))
		addresses = make([]string, len(hotels))
		phones    = make([]string, len(hotels))
	)

	for i, h := range hotels {
		ids[i] = h.ID
		names[i] = h.Name
		addresses[i] = h.Address
		phones[i] = h.Phone
	}

	return []driver.Value{pq.Array(ids), pq.Array(names), pq.Array(addresses), pq.Array(phones)}
}

func TestHotelRepository_GetHotelsByName(t *testing.T) {
	t.Parallel()

	hotels := []*model.HotelItem{
		{ID: "h1", Name: "Grand Hotel", Address: "1 Main St", Phone: "111"},
		{ID: "h2", Name: "Grand Plaza", Address: "2 Main St", Phone: "222"},
		{ID: "h3", Name: "Grand Resort", Address: "3 Main St", Phone: "333"},
	}

	nextPage, err := pagination.Encode(hotels[1].ID)
	require.NoError(t, err)

	tests := []struct {
		name         string
		params       model.FindHotelsParams
		expectations func(mock sqlmock.Sqlmock)
		want         *model.HotelsPage
		wantErr      error
		wantErrText  string
	}{
		{
			name:   "returns the hotels of the page without a next page token",
			params: model.FindHotelsParams{NamePattern: "grand", PageSize: 2},
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(selectHotelsQuery)).
					WithArgs("grand", "", 3).
					WillReturnRows(hotelRows(hotels[0], hotels[1]))
			},
			want: &model.HotelsPage{
				Hotels:   []*model.HotelItem{hotels[0], hotels[1]},
				PageSize: 2,
			},
		},
		{
			name:   "trims the extra row and returns a next page token",
			params: model.FindHotelsParams{NamePattern: "grand", AfterID: "h1", PageSize: 2},
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(selectHotelsQuery)).
					WithArgs("grand", "h1", 3).
					WillReturnRows(hotelRows(hotels[0], hotels[1], hotels[2]))
			},
			want: &model.HotelsPage{
				Hotels:   []*model.HotelItem{hotels[0], hotels[1]},
				PageSize: 2,
				NextPage: nextPage,
			},
		},
		{
			name:   "returns an empty page when nothing matches",
			params: model.FindHotelsParams{NamePattern: "nowhere", PageSize: 2},
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(selectHotelsQuery)).
					WithArgs("nowhere", "", 3).
					WillReturnRows(hotelRows())
			},
			want: &model.HotelsPage{Hotels: []*model.HotelItem{}, PageSize: 2},
		},
		{
			name:   "wraps query errors",
			params: model.FindHotelsParams{NamePattern: "grand", PageSize: 2},
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(selectHotelsQuery)).
					WithArgs("grand", "", 3).
					WillReturnError(errDefault)
			},
			wantErr:     errDefault,
			wantErrText: "find hotels by name:",
		},
		{
			name:   "wraps scan errors",
			params: model.FindHotelsParams{NamePattern: "grand", PageSize: 2},
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(selectHotelsQuery)).
					WithArgs("grand", "", 3).
					WillReturnRows(sqlmock.NewRows([]string{"id", "name", "address", "phone"}).
						AddRow("h1", "Grand Hotel", "1 Main St", nil))
			},
			wantErrText: "scan hotel:",
		},
		{
			name:   "wraps iteration errors",
			params: model.FindHotelsParams{NamePattern: "grand", PageSize: 2},
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta(selectHotelsQuery)).
					WithArgs("grand", "", 3).
					WillReturnRows(hotelRows(hotels[0]).RowError(0, errDefault))
			},
			wantErrText: "iterate hotels:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := newTestRepository(t, tt.expectations).GetHotelsByName(context.Background(), tt.params)

			switch {
			case tt.wantErr != nil:
				require.ErrorIs(t, err, tt.wantErr)
			case tt.wantErrText != "":
				require.ErrorContains(t, err, tt.wantErrText)
			default:
				require.NoError(t, err)
			}

			if tt.wantErr != nil || tt.wantErrText != "" {
				require.Nil(t, got)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHotelRepository_PutHotels(t *testing.T) {
	t.Parallel()

	hotels := []*model.HotelItem{
		{ID: "h1", Name: "Grand Hotel", Address: "1 Main St", Phone: "111"},
		{ID: "h2", Name: "Grand Plaza", Address: "2 Main St", Phone: "222"},
	}

	tests := []struct {
		name         string
		hotels       []*model.HotelItem
		expectations func(mock sqlmock.Sqlmock)
		wantErr      error
		wantErrText  string
	}{
		{
			name:   "inserts all hotels in a single statement",
			hotels: hotels,
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta(insertHotelsQuery)).
					WithArgs(hotelInsertArgs(hotels...)...).
					WillReturnResult(sqlmock.NewResult(0, 2))
			},
		},
		{
			name:         "skips the statement when there is nothing to insert",
			hotels:       nil,
			expectations: func(mock sqlmock.Sqlmock) {},
		},
		{
			name:   "wraps exec errors",
			hotels: hotels,
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta(insertHotelsQuery)).
					WithArgs(hotelInsertArgs(hotels...)...).
					WillReturnError(errDefault)
			},
			wantErr:     errDefault,
			wantErrText: "insert hotels:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := newTestRepository(t, tt.expectations).PutHotels(context.Background(), tt.hotels)

			switch {
			case tt.wantErr != nil:
				require.ErrorIs(t, err, tt.wantErr)
			case tt.wantErrText != "":
				require.ErrorContains(t, err, tt.wantErrText)
			default:
				require.NoError(t, err)
			}
		})
	}
}

func TestHotelRepository_DeleteAllHotels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		expectations func(mock sqlmock.Sqlmock)
		wantErr      error
		wantErrText  string
	}{
		{
			name: "deletes every hotel",
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta(deleteAllHotelsQuery)).
					WillReturnResult(sqlmock.NewResult(0, 5))
			},
		},
		{
			name: "wraps exec errors",
			expectations: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta(deleteAllHotelsQuery)).
					WillReturnError(errDefault)
			},
			wantErr:     errDefault,
			wantErrText: "delete all hotels:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := newTestRepository(t, tt.expectations).DeleteAllHotels(context.Background())

			switch {
			case tt.wantErr != nil:
				require.ErrorIs(t, err, tt.wantErr)
			case tt.wantErrText != "":
				require.ErrorContains(t, err, tt.wantErrText)
			default:
				require.NoError(t, err)
			}
		})
	}
}
