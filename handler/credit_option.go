package handler

import (
	"core/models"
	"core/service"
	"core/utils"
	"net/http"

	"github.com/labstack/echo"
)

type CreditOptionHandler struct {
	CreditOptionService service.CreditOptionService
}

func (h *CreditOptionHandler) GetCreditOptions(c echo.Context) error {
	data, err := h.CreditOptionService.GetOptionList()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, models.BasicResp{Message: err.Error()})
	}
	resp := models.BasicResp{
		Message: utils.Success,
		Data:    data,
	}

	return c.JSON(http.StatusOK, resp)
}
