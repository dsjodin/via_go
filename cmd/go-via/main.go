package main

import (
	"os"
	"strconv"

	"github.com/dsjodin/via_go/internal/api"
	"github.com/dsjodin/via_go/internal/config"
	ca "github.com/dsjodin/via_go/internal/crypto"
	"github.com/dsjodin/via_go/internal/dhcp"
	"github.com/dsjodin/via_go/internal/model"
	"github.com/dsjodin/via_go/internal/secrets"
	"github.com/dsjodin/via_go/internal/server"
	"github.com/dsjodin/via_go/internal/store"
	"github.com/dsjodin/via_go/internal/tftp"
	"github.com/dsjodin/via_go/internal/websockets"

	"github.com/gin-gonic/gin"

	"github.com/sirupsen/logrus"

	_ "github.com/dsjodin/via_go/docs"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// @title go-via
// @version 0.1
// @description VMware Imaging Appliances written in GO with full HTTP-REST API

// @BasePath /v1

func main() {

	logServer := websockets.NewLogServer()
	logrus.AddHook(logServer.Hook)
	logrus.WithFields(logrus.Fields{
		"version": version,
		"commit":  commit,
		"date":    date,
	}).Infof("Startup")

	// load config file
	conf := config.Load()

	//connect to database
	if conf.Debug {
		store.Connect(true)
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		store.Connect(false)
		gin.SetMode(gin.ReleaseMode)
	}

	//migrate all models
	//err := store.DB.AutoMigrate(&model.Pool{}, &model.Host{}, &model.Option{}, &model.DeviceClass{}, &model.Group{}, &model.Image{}, &model.User{})
	err := store.DB.AutoMigrate(&model.Host{}, &model.Option{}, &model.DeviceClass{}, &model.Group{}, &model.Image{}, &model.User{})
	if err != nil {
		logrus.Fatal(err)
	}

	//create admin user if it doesn't exist
	var adm model.User
	hp := api.HashAndSalt([]byte("VMware1!"))
	if res := store.DB.Where(model.User{UserForm: model.UserForm{Username: "admin"}}).Attrs(model.User{UserForm: model.UserForm{Password: hp}}).FirstOrCreate(&adm); res.Error != nil {
		logrus.Warning(res.Error)
	}

	// load secrets key
	key, err := secrets.Init()
	if err != nil {
		logrus.Fatalf("secrets: %v", err)
	}

	// DHCPd
	if !conf.DisableDhcp {
		for _, v := range conf.Network.Interfaces {
			go dhcp.Init(v)
		}
	}

	// TFTPd
	go tftp.Serve(conf)

	// HTTP routers
	https, efihttp, err := server.NewRouters(server.Options{
		Config:    conf,
		SecretKey: key,
		LogServer: logServer,
		Version:   server.Version{Version: version, Commit: commit, Date: date},
	})
	if err != nil {
		logrus.Fatal(err)
	}

	// check if ./cert/server.crt exists, if not we will create the folder, and initiate a new CA and a self-signed certificate
	crt, err := os.Stat("./cert/server.crt")
	if os.IsNotExist(err) {
		// create folder for certificates
		logrus.WithFields(logrus.Fields{
			"certificate": "server.crt does not exist, initiating new CA and creating self-signed ceritificate server.crt",
		}).Info("cert")
		if err := os.MkdirAll("cert", 0o755); err != nil {
			logrus.Fatalf("cert: could not create directory: %v", err)
		}
		ca.CreateCA()
		ca.CreateCert("./cert", "server", "server")
	} else {
		logrus.WithFields(logrus.Fields{
			crt.Name(): "server.crt found",
		}).Info("cert")
	}

	//enable HTTP for boot only on port 80
	logrus.WithFields(logrus.Fields{
		"port": ":80",
	}).Info("Webserver http")
	go func() {
		// The UEFI HTTP boot listener dying leaves hosts unable to boot, so
		// it must not fail silently.
		if err := efihttp.Run(":80"); err != nil {
			logrus.WithField("err", err).Error("Webserver http")
		}
	}()

	//enable HTTPS
	listen := ":" + strconv.Itoa(conf.Port)
	logrus.WithFields(logrus.Fields{
		"port": listen,
	}).Info("Webserver https")
	err = https.RunTLS(listen, "./cert/server.crt", "./cert/server.key")

	logrus.WithFields(logrus.Fields{
		"error": err,
	}).Error("Webserver")
}
