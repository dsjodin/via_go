package api

import (
	"errors"

	"github.com/dsjodin/via_go/internal/model"
	"github.com/gin-gonic/gin"
)

// Authentication failures are deliberately indistinguishable: telling a caller
// that a username exists but the password was wrong hands them half the answer.
var (
	errInvalidCredentials = errors.New("invalid username or password")
	errTooManyAttempts    = errors.New("too many failed attempts, try again later")
)

// ContextUsername is the gin context key holding the authenticated username.
const ContextUsername = "username"

func Error(c *gin.Context, status int, err error) {
	c.JSON(status, model.APIError{
		ErrorStatus:  status,
		ErrorMessage: err.Error(),
	})
}
