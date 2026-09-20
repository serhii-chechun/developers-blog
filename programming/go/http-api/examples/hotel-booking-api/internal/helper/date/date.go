package date

import (
	"fmt"
	"time"
)

// ParseOnly parses a date-only string in YYYY-MM-DD format.
func ParseOnly(value, name string) (time.Time, error) {
	if value == "" {
		return time.Time{}, fmt.Errorf("%s is required", name)
	}

	d, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must be in YYYY-MM-DD format", name)
	}

	return d, nil
}
