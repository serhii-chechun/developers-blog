package handler

import (
	"errors"
	"time"

	"hotel-booking-api/internal/room/model"

	"hotel-booking-api/internal/helper/date"
	"hotel-booking-api/internal/helper/pagination"

	"github.com/gin-gonic/gin"
)

func validateGetAllRequest(c *gin.Context) (model.FindAvailableRoomsParams, error) {
	var (
		query struct {
			GuestsCount  int    `form:"guests_count" binding:"required,gt=0"`
			CheckInDate  string `form:"check_in_date" binding:"required"`
			CheckOutDate string `form:"check_out_date" binding:"required"`
			HotelID      string `form:"hotel_id"`
			NextPage     string `form:"next_page"`
			PageSize     int    `form:"page_size"`
		}
		result model.FindAvailableRoomsParams
	)

	if err := c.ShouldBindQuery(&query); err != nil {
		return result, err
	}

	checkInDate, err := date.ParseOnly(query.CheckInDate, "check_in_date")
	if err != nil {
		return result, err
	}

	checkOutDate, err := date.ParseOnly(query.CheckOutDate, "check_out_date")
	if err != nil {
		return result, err
	}

	if checkInDate.Before(time.Now().Truncate(24 * time.Hour)) {
		return result, errors.New("check_in_date must not be in the past")
	}

	if !checkOutDate.After(checkInDate) {
		return result, errors.New("check_out_date must be after check_in_date")
	}

	result = model.FindAvailableRoomsParams{
		CheckInDate:  checkInDate.Format(time.DateOnly),
		CheckOutDate: checkOutDate.Format(time.DateOnly),
		GuestsCount:  query.GuestsCount,
		HotelID:      query.HotelID,
		PageSize:     pagination.NormalizeSize(query.PageSize),
	}

	if query.NextPage != "" {
		afterID, err := pagination.Decode(query.NextPage)
		if err != nil {
			return result, err
		}
		result.AfterID = afterID
	}

	return result, nil
}
