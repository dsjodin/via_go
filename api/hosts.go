package api

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/imdario/mergo"
	"github.com/maxiepax/go-via/db"
	"github.com/maxiepax/go-via/models"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ListHosts Get a list of all hosts
// @Summary Get all hosts
// @Tags hosts
// @Accept  json
// @Produce  json
// @Success 200 {array} models.Address
// @Failure 500 {object} models.APIError
// @Router /hosts [get]
func ListHosts(c *gin.Context) {
	var items []models.Host
	//if res := db.DB.Preload("Pool").Find(&items); res.Error != nil {
	if res := db.DB.Preload("Group").Find(&items); res.Error != nil {

		Error(c, http.StatusInternalServerError, res.Error) // 500
		return
	}
	c.JSON(http.StatusOK, items) // 200
}

// GetHost Get an existing Host
// @Summary Get an existing Host
// @Tags hosts
// @Accept  json
// @Produce  json
// @Param  id path int true "Host ID"
// @Success 200 {object} models.Host
// @Failure 400 {object} models.APIError
// @Failure 404 {object} models.APIError
// @Failure 500 {object} models.APIError
// @Router /hosts/{id} [get]
func GetHost(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, http.StatusBadRequest, err) // 400
		return
	}

	// Load the item
	var item models.Host
	//if res := db.DB.Preload("Pool").First(&item, id); res.Error != nil {
	if res := db.DB.Preload("Group").First(&item, id); res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			Error(c, http.StatusNotFound, fmt.Errorf("not found")) // 404
		} else {
			Error(c, http.StatusInternalServerError, res.Error) // 500
		}
		return
	}

	c.JSON(http.StatusOK, item) // 200
}

// SearchHost Search for an host
// @Summary Search for an host
// @Tags hosts
// @Accept  json
// @Produce  json
// @Param item body models.Host true "Fields to search for"
// @Success 200 {object} models.Host
// @Failure 400 {object} models.APIError
// @Failure 404 {object} models.APIError
// @Failure 500 {object} models.APIError
// @Router /hosts/search [post]
func SearchHost(c *gin.Context) {
	form := make(map[string]interface{})

	if err := c.ShouldBind(&form); err != nil {
		Error(c, http.StatusBadRequest, err) // 400
		return
	}

	query := db.DB

	for k, v := range form {
		query = query.Where(k, v)
	}

	// Load the item
	var item models.Host
	//if res := query.Preload("Pool").First(&item); res.Error != nil {
	if res := query.First(&item); res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			Error(c, http.StatusNotFound, fmt.Errorf("not found")) // 404
		} else {
			Error(c, http.StatusInternalServerError, res.Error) // 500
		}
		return
	}

	c.JSON(http.StatusOK, item) // 200
}

// CreateHost Create a new host
// @Summary Create a new host
// @Tags hosts
// @Accept  json
// @Produce  json
// @Param item body models.HostForm true "Add a host"
// @Success 200 {object} models.Host
// @Failure 400 {object} models.APIError
// @Failure 500 {object} models.APIError
// @Router /hosts [post]
func CreateHost(c *gin.Context) {
	var form models.HostForm

	if err := c.ShouldBind(&form); err != nil {
		Error(c, http.StatusBadRequest, err) // 400
		return
	}

	item := models.Host{HostForm: form}

	var group models.Group
	if res := db.DB.First(&group, "id = ?", form.GroupID); res.Error != nil {
		Error(c, http.StatusInternalServerError, res.Error) // 500
		return
	}

	// ensure the mac address is properly formated.
	mac, _ := net.ParseMAC(item.Mac)
	item.Mac = mac.String()

	// validate that provided IP (if any) is inside the group's network
	if strings.TrimSpace(item.IP) != "" {
		ipAddr, err := netip.ParseAddr(item.IP)
		if err != nil {
			Error(c, http.StatusBadRequest, fmt.Errorf("invalid IP address: %w", err))
			return
		}

		cidr, err := NetmaskToCIDR(group.Netmask)
		if err != nil {
			Error(c, http.StatusInternalServerError, fmt.Errorf("invalid group netmask: %w", err))
			return
		}

		networkAddr, err := NetworkAddress(group.Gateway, group.Netmask)
		if err != nil {
			Error(c, http.StatusInternalServerError, fmt.Errorf("invalid group gateway/netmask: %w", err))
			return
		}

		prefixStr := fmt.Sprintf("%s/%d", networkAddr, cidr)
		pfx, err := netip.ParsePrefix(prefixStr)
		if err != nil {
			Error(c, http.StatusInternalServerError, fmt.Errorf("failed to parse network prefix %s: %w", prefixStr, err))
			return
		}

		if !pfx.Contains(ipAddr) {
			Error(c, http.StatusBadRequest, fmt.Errorf("ip %s is not in group's network %s", item.IP, prefixStr))
			return
		}
	}

	// if ip address checks pass, continue to commit.
	if item.ID != 0 { // Save if its an existing item
		if res := db.DB.Save(&item); res.Error != nil {
			Error(c, http.StatusInternalServerError, res.Error) // 500
			return
		}
	} else { // Create a new item
		if res := db.DB.Create(&item); res.Error != nil {
			Error(c, http.StatusInternalServerError, res.Error) // 500
			return
		}
	}

	// Load a new version with relations
	//if res := db.DB.Preload("Pool").First(&item); res.Error != nil {
	if res := db.DB.First(&item); res.Error != nil {
		Error(c, http.StatusInternalServerError, res.Error) // 500
		return
	}

	c.JSON(http.StatusOK, item) // 200

	logrus.WithFields(logrus.Fields{
		"Hostname": item.Hostname,
		"Domain":   item.Domain,
		"IP":       item.IP,
		"MAC":      item.Mac,
		//"Pool ID":  item.PoolID,
		"Group ID": item.GroupID,
	}).Debug("host")
}

// UpdateHost Update an existing host
// @Summary Update an existing host
// @Tags hosts
// @Accept  json
// @Produce  json
// @Param  id path int true "Host ID"
// @Param  item body models.HostForm true "Update a host"
// @Success 200 {object} models.Host
// @Failure 400 {object} models.APIError
// @Failure 404 {object} models.APIError
// @Failure 500 {object} models.APIError
// @Router /hosts/{id} [patch]
func UpdateHost(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, http.StatusBadRequest, err) // 400
		return
	}

	// Load the form data
	var form models.HostForm
	if err := c.ShouldBind(&form); err != nil {
		Error(c, http.StatusBadRequest, err) // 400
		return
	}

	// Load the item
	var item models.Host
	if res := db.DB.First(&item, id); res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			Error(c, http.StatusNotFound, fmt.Errorf("not found")) // 404
		} else {
			Error(c, http.StatusInternalServerError, res.Error) // 500
		}
		return
	}

	// Merge the item and the form data
	if err := mergo.Merge(&item, models.Host{HostForm: form}, mergo.WithOverride); err != nil {
		Error(c, http.StatusInternalServerError, err) // 500
	}

	// Mergo doesn't overwrite 0 or false values, force set
	item.HostForm.Reimage = form.Reimage
	item.HostForm.Progress = form.Progress

	// Save it
	if res := db.DB.Save(&item); res.Error != nil {
		Error(c, http.StatusInternalServerError, res.Error) // 500
		return
	}

	// Load a new version with relations
	//if res := db.DB.Preload("Pool").First(&item); res.Error != nil {
	if res := db.DB.First(&item); res.Error != nil {
		Error(c, http.StatusInternalServerError, res.Error) // 500
		return
	}

	c.JSON(http.StatusOK, item) // 200
}

// DeleteHost Remove an existing host
// @Summary Remove an existing host
// @Tags hosts
// @Accept  json
// @Produce  json
// @Param  id path int true "Host ID"
// @Success 204
// @Failure 404 {object} models.APIError
// @Failure 500 {object} models.APIError
// @Router /hosts/{id} [delete]
func DeleteHost(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		Error(c, http.StatusBadRequest, err) // 400
		return
	}

	// Load the item
	var item models.Host
	if res := db.DB.First(&item, id); res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			Error(c, http.StatusNotFound, fmt.Errorf("not found")) // 404
		} else {
			Error(c, http.StatusInternalServerError, res.Error) // 500
		}
		return
	}

	// delete it
	if res := db.DB.Delete(&item); res.Error != nil {
		Error(c, http.StatusInternalServerError, res.Error) // 500
		return
	}

	c.JSON(http.StatusNoContent, gin.H{}) //204
}

// NetworkAddress returns the IPv4 network address for the provided gateway IP
// and netmask. netmask may be either a CIDR length ("24" or "/24") or dotted
// decimal ("255.255.255.0"). Returned network is the base network address
// (e.g. "192.168.1.0").
// Returns an error for invalid input or non-IPv4 addresses.
func NetworkAddress(gateway string, netmask string) (string, error) {
	ip := net.ParseIP(strings.TrimSpace(gateway)).To4()
	if ip == nil {
		return "", errors.New("invalid IPv4 gateway")
	}

	m := strings.TrimSpace(netmask)

	// accept "/24" or "24"
	if strings.HasPrefix(m, "/") {
		m = m[1:]
	}

	// If netmask is numeric (CIDR bits)
	if bits, err := strconv.Atoi(m); err == nil {
		if bits < 0 || bits > 32 {
			return "", errors.New("invalid CIDR mask length")
		}
		mask := net.CIDRMask(bits, 32)
		network := ip.Mask(mask)
		return network.String(), nil
	}

	// Otherwise expect dotted decimal like "255.255.255.0"
	pm := net.ParseIP(m)
	if pm == nil {
		return "", errors.New("invalid netmask format")
	}
	pm4 := pm.To4()
	if pm4 == nil {
		return "", errors.New("invalid IPv4 netmask")
	}
	mask := net.IPMask(pm4)
	network := ip.Mask(mask)
	return network.String(), nil
}

// NetmaskToCIDR converts a netmask in dotted-decimal ("255.255.255.0"),
// or CIDR formats ("24" or "/24") to the CIDR prefix length (e.g. 24).
func NetmaskToCIDR(maskStr string) (int, error) {
	m := strings.TrimSpace(maskStr)
	if strings.HasPrefix(m, "/") {
		m = strings.TrimPrefix(m, "/")
	}

	// If already numeric (CIDR)
	if bits, err := strconv.Atoi(m); err == nil {
		if bits < 0 || bits > 32 {
			return 0, fmt.Errorf("invalid CIDR length: %d", bits)
		}
		return bits, nil
	}

	// Expect dotted decimal
	ip := net.ParseIP(m)
	if ip == nil {
		return 0, fmt.Errorf("invalid netmask: %q", maskStr)
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return 0, fmt.Errorf("invalid IPv4 netmask: %q", maskStr)
	}
	ones, bits := net.IPMask(ip4).Size()
	if bits != 32 {
		return 0, fmt.Errorf("unexpected mask size: %d", bits)
	}
	return ones, nil
}
