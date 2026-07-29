package api

import (
	"net"
	"net/http"

	"github.com/dsjodin/via_go/internal/auth"
	"github.com/dsjodin/via_go/internal/model"
	"github.com/dsjodin/via_go/internal/store"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// Auth carries the shared session store and login throttle.
type Auth struct {
	Sessions *auth.Sessions
	Throttle *auth.Throttle

	// Secure marks the session cookie so browsers only send it over TLS. It
	// is off in tests, which speak plain HTTP to httptest.
	Secure bool
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// SessionResponse describes who the caller is.
type SessionResponse struct {
	Username string `json:"username"`
	// MustChangePassword tells the UI to force a password change before
	// letting the operator do anything else.
	MustChangePassword bool `json:"must_change_password"`
}

// throttleKey combines the account being attempted with where the attempt came
// from, so guessing at one account cannot lock out its real owner elsewhere.
func throttleKey(c *gin.Context, username string) string {
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		host = c.Request.RemoteAddr
	}
	return username + "|" + host
}

// Login exchanges credentials for a session cookie.
// @Summary Log in and receive a session cookie
// @Tags auth
// @Accept  json
// @Produce  json
// @Param item body loginRequest true "Credentials"
// @Success 200 {object} SessionResponse
// @Failure 400 {object} model.APIError
// @Failure 401 {object} model.APIError
// @Failure 429 {object} model.APIError
// @Router /login [post]
func (a *Auth) Login(c *gin.Context) {
	var form loginRequest
	if err := c.ShouldBind(&form); err != nil {
		Error(c, http.StatusBadRequest, err) // 400
		return
	}

	key := throttleKey(c, form.Username)
	if !a.Throttle.Allowed(key) {
		logrus.WithFields(logrus.Fields{
			"username": form.Username,
			"status":   "too many failed attempts",
		}).Warn("auth")
		Error(c, http.StatusTooManyRequests, errTooManyAttempts) // 429
		return
	}

	user, ok := a.verify(form.Username, form.Password, key)
	if !ok {
		Error(c, http.StatusUnauthorized, errInvalidCredentials) // 401
		return
	}

	token, err := a.Sessions.Create(user.Username)
	if err != nil {
		Error(c, http.StatusInternalServerError, err) // 500
		return
	}

	a.setCookie(c, token, int(auth.DefaultTTL.Seconds()))

	logrus.WithFields(logrus.Fields{
		"username": user.Username,
		"status":   "session started",
	}).Info("auth")

	c.JSON(http.StatusOK, SessionResponse{
		Username:           user.Username,
		MustChangePassword: user.MustChangePassword,
	}) // 200
}

// Logout invalidates the caller's session.
// @Summary Log out
// @Tags auth
// @Produce  json
// @Success 204
// @Router /logout [post]
func (a *Auth) Logout(c *gin.Context) {
	if token, err := c.Cookie(auth.CookieName); err == nil {
		a.Sessions.Destroy(token)
	}

	a.setCookie(c, "", -1)
	c.JSON(http.StatusNoContent, gin.H{}) // 204
}

// Session reports who the caller is.
// @Summary Describe the current session
// @Tags auth
// @Produce  json
// @Success 200 {object} SessionResponse
// @Failure 401 {object} model.APIError
// @Router /session [get]
func (a *Auth) Session(c *gin.Context) {
	username := c.GetString(ContextUsername)

	var user model.User
	if res := store.DB.Where("username = ?", username).First(&user); res.Error != nil {
		Error(c, http.StatusUnauthorized, errInvalidCredentials) // 401
		return
	}

	c.JSON(http.StatusOK, SessionResponse{
		Username:           user.Username,
		MustChangePassword: user.MustChangePassword,
	}) // 200
}

// verify checks credentials, recording the outcome against the throttle.
func (a *Auth) verify(username, password, key string) (model.User, bool) {
	var user model.User
	if res := store.DB.Where("username = ?", username).First(&user); res.Error != nil {
		logrus.WithFields(logrus.Fields{
			"username": username,
			"status":   "supplied username does not exist",
		}).Info("auth")
		a.Throttle.Fail(key)
		return user, false
	}

	if !ComparePasswords(user.Password, []byte(password), username) {
		logrus.WithFields(logrus.Fields{
			"username": username,
			"status":   "invalid password supplied",
		}).Info("auth")
		a.Throttle.Fail(key)
		return user, false
	}

	a.Throttle.Succeed(key)
	return user, true
}

func (a *Auth) setCookie(c *gin.Context, token string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:  auth.CookieName,
		Value: token,
		Path:  "/",
		// The session token must never be readable from JavaScript, so an
		// injected script cannot lift it.
		HttpOnly: true,
		Secure:   a.Secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   maxAge,
	})
}
