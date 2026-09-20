package date

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseOnly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		value       string
		field       string
		want        time.Time
		wantErrText string
	}{
		{
			name:  "parses a date only string",
			value: "2026-09-25",
			field: "check_in_date",
			want:  time.Date(2026, time.September, 25, 0, 0, 0, 0, time.UTC),
		},
		{
			name:  "parses the last day of a leap february",
			value: "2024-02-29",
			field: "check_out_date",
			want:  time.Date(2024, time.February, 29, 0, 0, 0, 0, time.UTC),
		},
		{
			name:        "reports an empty value as required",
			value:       "",
			field:       "check_out_date",
			wantErrText: "check_out_date is required",
		},
		{
			name:        "rejects a slash separated date",
			value:       "25/09/2026",
			field:       "check_in_date",
			wantErrText: "check_in_date must be in YYYY-MM-DD format",
		},
		{
			name:        "rejects an unpadded date",
			value:       "2026-9-25",
			field:       "check_in_date",
			wantErrText: "check_in_date must be in YYYY-MM-DD format",
		},
		{
			name:        "rejects a month that does not exist",
			value:       "2026-13-01",
			field:       "check_in_date",
			wantErrText: "check_in_date must be in YYYY-MM-DD format",
		},
		{
			name:        "rejects a day that does not exist",
			value:       "2026-02-30",
			field:       "check_in_date",
			wantErrText: "check_in_date must be in YYYY-MM-DD format",
		},
		{
			name:        "rejects february the 29th of a common year",
			value:       "2026-02-29",
			field:       "check_in_date",
			wantErrText: "check_in_date must be in YYYY-MM-DD format",
		},
		{
			name:        "rejects a timestamp",
			value:       "2026-09-25T10:00:00Z",
			field:       "check_in_date",
			wantErrText: "check_in_date must be in YYYY-MM-DD format",
		},
		{
			name:        "rejects a padded value",
			value:       " 2026-09-25",
			field:       "check_in_date",
			wantErrText: "check_in_date must be in YYYY-MM-DD format",
		},
		{
			name:        "rejects an empty name for an unparsable value",
			value:       "whenever",
			field:       "",
			wantErrText: " must be in YYYY-MM-DD format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseOnly(tt.value, tt.field)

			if tt.wantErrText != "" {
				require.ErrorContains(t, err, tt.wantErrText)
			} else {
				require.NoError(t, err)
			}

			assert.Truef(t, tt.want.Equal(got), "want instant %v, got %v", tt.want, got)
			assert.Equal(t, tt.want.Location(), got.Location())
		})
	}
}
