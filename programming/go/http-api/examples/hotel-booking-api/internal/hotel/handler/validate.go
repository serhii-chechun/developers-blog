package handler

import (
	"hotel-booking-api/internal/hotel/model"

	"hotel-booking-api/internal/helper/pagination"

	"github.com/gin-gonic/gin"
)

func validateGetAllRequest(c *gin.Context) (model.FindHotelsParams, error) {
	var (
		query struct {
			NamePattern string `form:"name_pattern"`
			NextPage    string `form:"next_page"`
			PageSize    int    `form:"page_size"`
		}
		result model.FindHotelsParams
	)

	if err := c.ShouldBindQuery(&query); err != nil {
		return result, err
	}

	result = model.FindHotelsParams{
		NamePattern: query.NamePattern,
		PageSize:    pagination.NormalizeSize(query.PageSize),
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
