package api

import (
	"net/http"

	"github.com/dsjodin/via_go/internal/model"
	"github.com/dsjodin/via_go/internal/store"
	"github.com/gin-gonic/gin"
	"github.com/imdario/mergo"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

// Reads and deletes are plain persistence; create and update are not, because
// the password has to be hashed on the way in and must not be touched when a
// request does not carry one.

// ListUsers Get a list of all users
// @Summary Get all users
// @Tags users
// @Accept  json
// @Produce  json
// @Success 200 {array} model.User
// @Failure 500 {object} model.APIError
// @Router /users [get]
func ListUsers(c *gin.Context) { List[model.User](c) }

// GetUser Get an existing user
// @Summary Get an existing user
// @Tags users
// @Accept  json
// @Produce  json
// @Param  id path int true "User ID"
// @Success 200 {object} model.User
// @Failure 400 {object} model.APIError
// @Failure 404 {object} model.APIError
// @Failure 500 {object} model.APIError
// @Router /users/{id} [get]
func GetUser(c *gin.Context) { Get[model.User](c) }

// SearchUser Search for a user
// @Summary Search for a user
// @Tags users
// @Accept  json
// @Produce  json
// @Param item body model.User true "Fields to search for"
// @Success 200 {object} model.User
// @Failure 400 {object} model.APIError
// @Failure 404 {object} model.APIError
// @Failure 500 {object} model.APIError
// @Router /users/search [post]
func SearchUser(c *gin.Context) { Search[model.User](c) }

// DeleteUser Remove an existing user
// @Summary Remove an existing user
// @Tags users
// @Accept  json
// @Produce  json
// @Param  id path int true "User ID"
// @Success 204
// @Failure 404 {object} model.APIError
// @Failure 500 {object} model.APIError
// @Router /users/{id} [delete]
func DeleteUser(c *gin.Context) { Delete[model.User](c) }

// CreateUser Create a new user
// @Summary Create a new user
// @Tags users
// @Accept  json
// @Produce  json
// @Param item body model.UserRequest true "Add an user"
// @Success 200 {object} model.User
// @Failure 400 {object} model.APIError
// @Failure 500 {object} model.APIError
// @Router /users [post]
func CreateUser(c *gin.Context) {
	var form model.UserRequest
	if err := c.ShouldBind(&form); err != nil {
		Error(c, http.StatusBadRequest, err) // 400
		return
	}

	item := model.User{
		UserForm: form.UserForm,
		Password: HashAndSalt([]byte(form.Password)),
	}

	if res := store.DB.Create(&item); res.Error != nil {
		Error(c, http.StatusInternalServerError, res.Error) // 500
		return
	}

	c.JSON(http.StatusOK, item) // 200
}

// UpdateUser Update an existing user
// @Summary Update an existing user
// @Tags users
// @Accept  json
// @Produce  json
// @Param  id path int true "User ID"
// @Param  item body model.UserRequest true "Update a user"
// @Success 200 {object} model.User
// @Failure 400 {object} model.APIError
// @Failure 404 {object} model.APIError
// @Failure 500 {object} model.APIError
// @Router /users/{id} [patch]
func UpdateUser(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}

	var form model.UserRequest
	if err := c.ShouldBind(&form); err != nil {
		Error(c, http.StatusBadRequest, err) // 400
		return
	}

	item, ok := load[model.User](c, id)
	if !ok {
		return
	}

	if err := mergo.Merge(&item, model.User{UserForm: form.UserForm}, mergo.WithOverride); err != nil {
		Error(c, http.StatusInternalServerError, err) // 500
		return
	}

	// Only hash when a new password was actually supplied. This used to hash
	// item.Password unconditionally, which on a request that did not carry one
	// meant re-hashing the stored bcrypt hash — quietly destroying the
	// password. Changing the admin account's email locked you out of the
	// appliance.
	if form.Password != "" {
		item.Password = HashAndSalt([]byte(form.Password))
	}

	if res := store.DB.Save(&item); res.Error != nil {
		Error(c, http.StatusInternalServerError, res.Error) // 500
		return
	}

	c.JSON(http.StatusOK, item) // 200
}

func HashAndSalt(pwd []byte) string {
	// Generate hashed and salted password
	hash, err := bcrypt.GenerateFromPassword(pwd, bcrypt.MinCost)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error": err,
		}).Error("couldnt salt and hash password")
	}
	return string(hash)
}

func ComparePasswords(hashedPwd string, plainPwd []byte, username string) bool {
	// compare a password to the hashed and salted value
	byteHash := []byte(hashedPwd)
	err := bcrypt.CompareHashAndPassword(byteHash, plainPwd)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"username": username,
			"error":    err,
		}).Error("invalid password supplied")
		return false
	}
	return true
}
