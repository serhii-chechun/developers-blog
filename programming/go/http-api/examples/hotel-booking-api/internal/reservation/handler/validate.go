package handler

import (
	"errors"
	"time"

	"hotel-booking-api/internal/reservation/model"

	"hotel-booking-api/internal/helper/date"

	"github.com/gin-gonic/gin"
)

func validateCreateRequest(c *gin.Context) (model.ReservationItem, error) {
	var (
		form struct {
			HotelID       string   `form:"hotel_id" json:"hotel_id" binding:"required"`
			CheckInDate   string   `form:"check_in_date" json:"check_in_date" binding:"required"`
			CheckOutDate  string   `form:"check_out_date" json:"check_out_date" binding:"required"`
			GuestFullName string   `form:"guest_full_name" json:"guest_full_name" binding:"required"`
			GuestEmail    string   `form:"guest_email" json:"guest_email" binding:"required,email"`
			GuestsCount   int      `form:"guests_count" json:"guests_count" binding:"required,gt=0"`
			RoomIDs       []string `form:"room_ids" json:"room_ids"`
		}
		result model.ReservationItem
	)

	if err := c.ShouldBind(&form); err != nil {
		return result, err
	}

	checkInDate, err := date.ParseOnly(form.CheckInDate, "check_in_date")
	if err != nil {
		return result, err
	}

	checkOutDate, err := date.ParseOnly(form.CheckOutDate, "check_out_date")
	if err != nil {
		return result, err
	}

	if checkInDate.Before(time.Now().Truncate(24 * time.Hour)) {
		return result, errors.New("check_in_date must not be in the past")
	}

	if !checkOutDate.After(checkInDate) {
		return result, errors.New("check_out_date must be after check_in_date")
	}

	result = model.ReservationItem{
		HotelID:       form.HotelID,
		CheckInDate:   checkInDate.Format(time.DateOnly),
		CheckOutDate:  checkOutDate.Format(time.DateOnly),
		GuestFullName: form.GuestFullName,
		GuestEmail:    form.GuestEmail,
		GuestsCount:   form.GuestsCount,
		RoomIDs:       form.RoomIDs,
	}

	return result, nil
}
