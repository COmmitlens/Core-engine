package handler

import (
	"core/models"
	"core/service"
	"net/mail"

	"github.com/labstack/echo"
)

type WaitlistHandler struct {
	WaitlistService service.WaitlistService
}

func (waitlistHandler *WaitlistHandler) AddToWaitlist(c echo.Context) error {
	param := models.WaitlistParam{}
	err := c.Bind(&param)
	if err != nil {
		return c.JSON(400, models.BasicRespMesg{Message: "Invalid request"})
	}
	if param.Name == "" || param.Company == "" || param.Email == "" || param.Message == "" {
		return c.JSON(400, models.BasicRespMesg{Message: "All fields are required"})
	}
	if _, err := mail.ParseAddress(param.Email); err != nil {
		return c.JSON(400, models.BasicRespMesg{Message: "Invalid email address"})
	}
	if len(param.Message) > 500 {
		return c.JSON(400, models.BasicRespMesg{Message: "Message must not exceed 500 characters"})
	}
	err = waitlistHandler.WaitlistService.AddToWaitlist(param)
	if err != nil {
		return c.JSON(500, models.BasicRespMesg{Message: "Failed to add to waitlist"})
	}
	return c.JSON(200, models.BasicRespMesg{Message: "Successfully added to waitlist"})

}
