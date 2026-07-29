package api

import (
	"github.com/dsjodin/via_go/internal/model"
	"github.com/gin-gonic/gin"
)

func Error(c *gin.Context, status int, err error) {
	c.JSON(status, model.APIError{
		ErrorStatus:  status,
		ErrorMessage: err.Error(),
	})
}
