package api

import (
	"github.com/dsjodin/via_go/models"
	"github.com/gin-gonic/gin"
)

func Error(c *gin.Context, status int, err error) {
	c.JSON(status, models.APIError{
		ErrorStatus:  status,
		ErrorMessage: err.Error(),
	})
}
