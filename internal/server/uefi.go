package server

import (
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/dsjodin/via_go/internal/boot"
	"github.com/dsjodin/via_go/internal/config"
	"github.com/dsjodin/via_go/internal/model"
	"github.com/dsjodin/via_go/internal/store"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm/clause"
)

func Files(conf *config.Config) func(c *gin.Context) {
	return func(c *gin.Context) {
		filepath := c.Param("filepath")

		//ip := net.ParseIP(c.ClientIP())
		ip, _, _ := net.SplitHostPort(c.Request.RemoteAddr)
		//get the object that correlates with the ip
		var host model.Host
		store.DB.Preload(clause.Associations).First(&host, "ip = ?", ip)

		//get the image info that correlates with the host
		var image model.Image
		store.DB.First(&image, "id = ?", host.Group.ImageID)

		switch filepath {
		case "/mboot.efi":
			serveLoader(c, &host, boot.Mboot, image.Path, ip, "mboot.efi", 10)
			return
		case "/crypto64.efi":
			serveLoader(c, &host, boot.Crypto64, image.Path, ip, "crypto64.efi", 12)
			return
		case "boot.cfg", "/boot.cfg":
			localAddr, _ := c.Request.Context().Value(http.LocalAddrContextKey).(net.Addr)
			localipport := localAddr.String()
			localip, _, _ := net.SplitHostPort(localipport)
			remoteip, _, _ := net.SplitHostPort(c.Request.RemoteAddr)
			bootconfig, err := serveBootCfg(filepath, host, image, conf, localip, remoteip)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"error": err,
				}).Error("boot.cfg")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate boot.cfg"})
				return
			}
			c.Data(http.StatusOK, "application/octet-stream", bootconfig)
			return
		default:
			//if no case matches, chroot to /images
			if _, err := os.Stat(image.Path + filepath); err == nil {
				filepath = image.Path + filepath
				logrus.WithFields(logrus.Fields{
					"lowercase file": filepath,
				}).Debug("uefi-https")
			} else {
				filepath = image.Path + strings.ToUpper(filepath)
				logrus.WithFields(logrus.Fields{
					"uppercase file": filepath,
				}).Debug("uefi-https")
			}
		}
		c.File(filepath)
	}
}

// serveLoader sends one of the loader binaries from inside the host's image and
// records the progress that reaching it represents.
func serveLoader(c *gin.Context, host *model.Host, locate func(string) (string, error), imagePath, ip, name string, progress int) {
	logrus.WithFields(logrus.Fields{
		ip: "requesting " + name,
	}).Info("uefi-https")

	path, err := locate(imagePath)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"image": imagePath,
			"error": err,
		}).Error("uefi-https")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not locate " + name})
		return
	}

	logrus.WithFields(logrus.Fields{
		"id":           host.ID,
		"percentage":   progress,
		"progresstext": name,
	}).Info("progress")
	host.Progress = progress
	host.Progresstext = name
	store.DB.Save(host)

	c.File(path)
}

func serveBootCfg(filepath string, host model.Host, image model.Image, conf *config.Config, localip string, remoteip string) ([]byte, error) {
	var group model.Group
	store.DB.Preload(clause.Associations).First(&group, "id = ?", host.GroupID)

	logrus.WithFields(logrus.Fields{
		remoteip: "requesting boot.cfg",
	}).Info("uefi-https")
	logrus.WithFields(logrus.Fields{
		"id":           host.ID,
		"percentage":   15,
		"progresstext": "installation",
	}).Info("progress")
	host.Progress = 15
	host.Progresstext = "installation"
	store.DB.Save(&host)

	path, err := boot.Config(image.Path)
	if err != nil {
		logrus.Warn(err)
		return nil, err
	}

	bc, err := os.ReadFile(path)
	if err != nil {
		logrus.Warn(err)
		return nil, err
	}

	out := boot.RenderConfig(bc, boot.Params{
		// The loader fetches the rest of the image back over HTTPS from here.
		Prefix:       boot.HTTPPrefix("https", localip, conf.Port),
		KickstartURL: boot.KickstartURL(localip, conf.Port),
		Mac:          host.Mac,
		IP:           host.IP,
		Netmask:      group.Netmask,
		Gateway:      group.Gateway,
		Vlan:         host.Group.Vlan,
		// Matches --forceunsupportedinstall in the kickstart template.
		AllowLegacyCPU: true,
	})

	logrus.WithFields(logrus.Fields{
		"file": filepath,
	}).Info("uefi-https")

	return out, nil
}
