package model

type (
	// HotelItem represents a hotel record with its basic information.
	HotelItem struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Address string `json:"address"`
		Phone   string `json:"phone"`
	}
)

type (
	// FindHotelsParams represents the parameters required to find hotels by name.
	FindHotelsParams struct {
		NamePattern string
		AfterID     string
		PageSize    int
	}

	// HotelsPage represents a page of hotels together with its pagination metadata.
	HotelsPage struct {
		Hotels   []*HotelItem `json:"hotels"`
		PageSize int          `json:"page_size"`
		NextPage string       `json:"next_page,omitempty"`
	}
)
