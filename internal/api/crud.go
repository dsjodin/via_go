package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/dsjodin/via_go/internal/store"
	"github.com/gin-gonic/gin"
	"github.com/imdario/mergo"
	"gorm.io/gorm"
)

// The handlers in this file implement the create/read/update/delete behaviour
// every resource shares. Each resource keeps a named function carrying its
// swagger annotations that delegates here, so the generated OpenAPI spec still
// describes the API while the bodies live in one place.
//
// Anything a resource does beyond plain persistence — encrypting a group
// password, hashing a user password, extracting an uploaded ISO — stays in
// that resource's own handler.

// List returns every record of a type.
func List[T any](c *gin.Context) {
	var items []T
	if res := store.DB.Find(&items); res.Error != nil {
		Error(c, http.StatusInternalServerError, res.Error) // 500
		return
	}

	c.JSON(http.StatusOK, items) // 200
}

// Get returns one record by id.
func Get[T any](c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}

	item, ok := load[T](c, id)
	if !ok {
		return
	}

	c.JSON(http.StatusOK, item) // 200
}

// Search returns the first record matching every supplied field.
//
// The request body is a map of column name to value. Keys are passed to gorm
// as map keys rather than as condition strings, so gorm quotes them as column
// identifiers. The previous implementation did store.DB.Where(key, value) with
// the key straight from the request, which let a caller supply a SQL fragment:
// {"name = ? OR 1=1": "nope"} matched every row, and a subquery could read any
// column of any table, including the encrypted group passwords.
func Search[T any](c *gin.Context) {
	form := make(map[string]any)
	if err := c.ShouldBind(&form); err != nil {
		Error(c, http.StatusBadRequest, err) // 400
		return
	}

	if len(form) == 0 {
		Error(c, http.StatusBadRequest, fmt.Errorf("no search fields supplied")) // 400
		return
	}

	var item T
	res := store.DB.Where(form).First(&item)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			Error(c, http.StatusNotFound, fmt.Errorf("not found")) // 404
		} else {
			// An unknown column reaches here as a database error rather than a
			// silently broad match.
			Error(c, http.StatusBadRequest, res.Error) // 400
		}
		return
	}

	c.JSON(http.StatusOK, item) // 200
}

// Create persists a new record built from the bound request body.
//
// assemble turns the request shape into the stored shape; it is a function
// rather than a type constraint because the two are related by struct
// embedding, which generics cannot express.
func Create[T, F any](c *gin.Context, assemble func(F) T) {
	var form F
	if err := c.ShouldBind(&form); err != nil {
		Error(c, http.StatusBadRequest, err) // 400
		return
	}

	item := assemble(form)
	if res := store.DB.Create(&item); res.Error != nil {
		Error(c, http.StatusInternalServerError, res.Error) // 500
		return
	}

	c.JSON(http.StatusOK, item) // 200
}

// Update merges the bound request body over an existing record.
func Update[T, F any](c *gin.Context, assemble func(F) T) {
	id, ok := pathID(c)
	if !ok {
		return
	}

	var form F
	if err := c.ShouldBind(&form); err != nil {
		Error(c, http.StatusBadRequest, err) // 400
		return
	}

	item, ok := load[T](c, id)
	if !ok {
		return
	}

	if err := mergo.Merge(&item, assemble(form), mergo.WithOverride); err != nil {
		Error(c, http.StatusInternalServerError, err) // 500
		return
	}

	if res := store.DB.Save(&item); res.Error != nil {
		Error(c, http.StatusInternalServerError, res.Error) // 500
		return
	}

	c.JSON(http.StatusOK, item) // 200
}

// Delete removes a record by id.
func Delete[T any](c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}

	item, ok := load[T](c, id)
	if !ok {
		return
	}

	if res := store.DB.Delete(&item); res.Error != nil {
		Error(c, http.StatusInternalServerError, res.Error) // 500
		return
	}

	c.JSON(http.StatusNoContent, gin.H{}) // 204
}

// pathID reads the :id path parameter, writing the 400 itself.
func pathID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, http.StatusBadRequest, err) // 400
		return 0, false
	}
	return id, true
}

// load fetches one record by id, writing the 404 or 500 itself. The boolean
// reports whether the caller should carry on.
func load[T any](c *gin.Context, id int) (T, bool) {
	var item T
	if res := store.DB.First(&item, id); res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			Error(c, http.StatusNotFound, fmt.Errorf("not found")) // 404
		} else {
			Error(c, http.StatusInternalServerError, res.Error) // 500
		}
		return item, false
	}

	return item, true
}
