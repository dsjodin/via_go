package api

import (
	"net/http"

	"github.com/dsjodin/via_go/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// Middleware authenticates a request by session cookie or by HTTP basic auth.
//
// Both are supported on purpose. The browser uses a session cookie, which is
// httpOnly and can be invalidated server side. Automation uses basic auth,
// which is what the example scripts and the documented "everything you can do
// in the UI, you can do via automation" promise rely on. Dropping it would
// break every existing script.
//
// Basic auth is throttled the same way login is, since it is otherwise a
// guessing endpoint on every route.
func (a *Auth) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if token, err := c.Cookie(auth.CookieName); err == nil {
			if username, ok := a.Sessions.Lookup(token); ok {
				c.Set(ContextUsername, username)
				c.Next()
				return
			}
		}

		username, password, hasAuth := c.Request.BasicAuth()
		if !hasAuth {
			logrus.WithFields(logrus.Fields{
				"login": "unauthorized request",
			}).Info("auth")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		key := throttleKey(c, username)
		if !a.Throttle.Allowed(key) {
			logrus.WithFields(logrus.Fields{
				"username": username,
				"status":   "too many failed attempts",
			}).Warn("auth")
			c.AbortWithStatus(http.StatusTooManyRequests)
			return
		}

		user, ok := a.verify(username, password, key)
		if !ok {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		logrus.WithFields(logrus.Fields{
			"username": user.Username,
			"status":   "successfully authenticated",
		}).Debug("auth")

		c.Set(ContextUsername, user.Username)
		c.Next()
	}
}
