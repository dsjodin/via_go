package api

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	//"net/url"
	"strconv"

	"github.com/dsjodin/via_go/internal/model"
	"github.com/dsjodin/via_go/internal/store"
	"github.com/gin-gonic/gin"
	//"github.com/dsjodin/via_go/internal/secrets"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm/clause"
)

func PostConfig(key string) func(c *gin.Context) {
	return func(c *gin.Context) {
		var item model.Host
		host, _, _ := net.SplitHostPort(c.Request.RemoteAddr)

		if res := store.DB.Preload(clause.Associations).Where("ip = ?", host).First(&item); res.Error != nil {
			Error(c, http.StatusInternalServerError, res.Error) // 500
			return
		}

		c.JSON(http.StatusOK, item) // 200

		logrus.Info("ks config done!")

		go ProvisioningWorker(item, key)
	}
}

func PostConfigID(key string) func(c *gin.Context) {
	return func(c *gin.Context) {
		var item model.Host

		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			Error(c, http.StatusBadRequest, err) // 400
			return
		}

		if res := store.DB.Preload(clause.Associations).Where("id = ?", id).First(&item); res.Error != nil {
			Error(c, http.StatusInternalServerError, res.Error) // 500
			return
		}

		c.JSON(http.StatusOK, item) // 200

		logrus.Info("Manual PostConfig of host" + item.Hostname + "started!")

		go ProvisioningWorker(item, key)
	}
}

func ProvisioningWorker(item model.Host, key string) {

	//create empty model and load it with the json content from database
	options := model.GroupOptions{}
	if err := json.Unmarshal(item.Group.Options, &options); err != nil {
		logrus.WithField("err", err).Warn("could not parse group options, treating them as unset")
	}
	logrus.WithFields(logrus.Fields{
		"Started worker for ": item.Hostname,
	}).Debug("host")

	// decrypt login password
	//decryptedPassword := secrets.Decrypt(item.Group.Password, key)

	// connection info
	/*
		url := &url.URL{
			Scheme: "https",
			Host:   item.IP,
			Path:   "sdk",
			User:   url.UserPassword("root", decryptedPassword),
		}
	*/

	logrus.WithFields(logrus.Fields{
		"id":           item.ID,
		"percentage":   75,
		"progresstext": "customization",
	}).Info("progress")
	item.Progress = 75
	item.Progresstext = "customization"
	store.DB.Save(&item)

	// ensure that host has enough time to boot, and for SOAP API to respond
	/*
		var c *govmomi.Client
		var err error
		ctx := context.Background()
		i := 1
		timeout := 360

		for {
			if i > timeout {
				logrus.WithFields(logrus.Fields{
					"IP":     item.IP,
					"status": "timeout exceeded, failing postconfig",
				}).Info("postconfig")
				return
			}

			if res := store.DB.First(&item, item.ID); res.Error != nil {
				logrus.WithFields(logrus.Fields{
					"IP":  item.IP,
					"err": res.Error,
				}).Error("postconfig failed to read state")
				return
			}

			if item.Progress == 0 {
				logrus.WithFields(logrus.Fields{
					"IP": item.IP,
				}).Error("postconfig terminated")
				return
			}

			c, err = govmomi.NewClient(ctx, url, true)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"IP":        item.IP,
					"status":    "Hosts SOAP API not ready yet, retrying",
					"retry":     i,
					"retry max": timeout,
				}).Info("postconfig")
				logrus.WithFields(logrus.Fields{
					"IP":        item.IP,
					"status":    "Hosts SOAP API not ready yet, retrying",
					"retry":     i,
					"retry max": timeout,
					"err":       err,
				}).Debug("postconfig")
				i += 1
				<-time.After(time.Second * 10)
				continue
			}
			break
		}
	*/

	//postconfig completed
	logrus.WithFields(logrus.Fields{
		"IP":         item.IP,
		"postconfig": "postconfig completed",
	}).Info("postconfig")

	logrus.WithFields(logrus.Fields{
		"id":           item.ID,
		"percentage":   100,
		"progresstext": "completed",
	}).Info("progress")
	item.Progress = 100
	item.Progresstext = "completed"
	store.DB.Save(&item)

	//send callback if set
	if item.Group.CallbackURL != "" {
		err := callback(item.Group.CallbackURL, item)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"postconfig": err,
			}).Info("")
			return
		}
	}

}

func callback(url string, data model.Host) error {
	//remove password
	data.Group.Password = ""
	//convert model to json
	json_data, err := json.Marshal(data)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"postconfig": err,
		}).Info("")
		return err
	}
	//convert json string to io.reader
	reader := bytes.NewReader(json_data)

	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPost, url, reader)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"postconfig": err,
		}).Info("")
		return err
	}
	_, err = client.Do(req)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"postconfig": err,
		}).Info("")
		return err
	}
	logrus.WithFields(logrus.Fields{
		"IP":       data.IP,
		"callback": data.Group.CallbackURL,
	}).Info("progress")
	return nil
}
