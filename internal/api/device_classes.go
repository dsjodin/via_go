package api

import (
	"github.com/dsjodin/via_go/internal/model"
	"github.com/gin-gonic/gin"
)

// Device classes are plain persistence, so every handler delegates to the
// generic implementations in crud.go. The functions exist to carry the swagger
// annotations and to give the router something named to register.

func newDeviceClass(f model.DeviceClassForm) model.DeviceClass {
	return model.DeviceClass{DeviceClassForm: f}
}

// ListDeviceClasses Get a list of all device classes
// @Summary Get all device classes
// @Tags device_classes
// @Accept  json
// @Produce  json
// @Success 200 {array} model.DeviceClass
// @Failure 500 {object} model.APIError
// @Router /device_classes [get]
func ListDeviceClasses(c *gin.Context) { List[model.DeviceClass](c) }

// GetDeviceClass Get an existing device class
// @Summary Get an existing device class
// @Tags device_classes
// @Accept  json
// @Produce  json
// @Param  id path int true "DeviceClass ID"
// @Success 200 {object} model.DeviceClass
// @Failure 400 {object} model.APIError
// @Failure 404 {object} model.APIError
// @Failure 500 {object} model.APIError
// @Router /device_classes/{id} [get]
func GetDeviceClass(c *gin.Context) { Get[model.DeviceClass](c) }

// SearchDeviceClass Search for an device class
// @Summary Search for an device class
// @Tags device_classes
// @Accept  json
// @Produce  json
// @Param item body model.DeviceClass true "Fields to search for"
// @Success 200 {object} model.DeviceClass
// @Failure 400 {object} model.APIError
// @Failure 404 {object} model.APIError
// @Failure 500 {object} model.APIError
// @Router /device_classes/search [post]
func SearchDeviceClass(c *gin.Context) { Search[model.DeviceClass](c) }

// CreateDeviceClass Create a new device class
// @Summary Create a new device class
// @Tags device_classes
// @Accept  json
// @Produce  json
// @Param item body model.DeviceClassForm true "Add an device class"
// @Success 200 {object} model.DeviceClass
// @Failure 400 {object} model.APIError
// @Failure 500 {object} model.APIError
// @Router /device_classes [post]
func CreateDeviceClass(c *gin.Context) { Create(c, newDeviceClass) }

// UpdateDeviceClass Update an existing device class
// @Summary Update an existing device class
// @Tags device_classes
// @Accept  json
// @Produce  json
// @Param  id path int true "DeviceClass ID"
// @Param  item body model.DeviceClassForm true "Update an ip device class"
// @Success 200 {object} model.DeviceClass
// @Failure 400 {object} model.APIError
// @Failure 404 {object} model.APIError
// @Failure 500 {object} model.APIError
// @Router /device_classes/{id} [patch]
func UpdateDeviceClass(c *gin.Context) { Update(c, newDeviceClass) }

// DeleteDeviceClass Remove an existing device class
// @Summary Remove an existing device class
// @Tags device_classes
// @Accept  json
// @Produce  json
// @Param  id path int true "DeviceClass ID"
// @Success 204
// @Failure 404 {object} model.APIError
// @Failure 500 {object} model.APIError
// @Router /device_classes/{id} [delete]
func DeleteDeviceClass(c *gin.Context) { Delete[model.DeviceClass](c) }
