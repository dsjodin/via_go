//go:generate bash -c "go get github.com/swaggo/swag/cmd/swag && swag init"
//go:generate bash -c "cd ui && npm ci && npm run build && cd .. && rm -rf webui/dist && cp -r ui/out webui/dist"

package main

import (
	"net/http"
	"os"
	"strconv"

	"github.com/dsjodin/via_go/api"
	"github.com/dsjodin/via_go/config"
	ca "github.com/dsjodin/via_go/crypto"
	"github.com/dsjodin/via_go/db"
	"github.com/dsjodin/via_go/dhcpd"
	"github.com/dsjodin/via_go/models"
	"github.com/dsjodin/via_go/secrets"
	"github.com/dsjodin/via_go/uefi"
	"github.com/dsjodin/via_go/websockets"
	"github.com/dsjodin/via_go/webui"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/gin-contrib/cors"
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
		db.Connect(true)
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		db.Connect(false)
		gin.SetMode(gin.ReleaseMode)
	}

	//migrate all models
	//err := db.DB.AutoMigrate(&models.Pool{}, &models.Host{}, &models.Option{}, &models.DeviceClass{}, &models.Group{}, &models.Image{}, &models.User{})
	err := db.DB.AutoMigrate(&models.Host{}, &models.Option{}, &models.DeviceClass{}, &models.Group{}, &models.Image{}, &models.User{})
	if err != nil {
		logrus.Fatal(err)
	}

	//create admin user if it doesn't exist
	var adm models.User
	hp := api.HashAndSalt([]byte("VMware1!"))
	if res := db.DB.Where(models.User{UserForm: models.UserForm{Username: "admin"}}).Attrs(models.User{UserForm: models.UserForm{Password: hp}}).FirstOrCreate(&adm); res.Error != nil {
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
			go dhcpd.Init(v)
		}
	}

	// TFTPd
	go TFTPd(conf)

	//HTTPS REST API
	https := gin.New()
	https.Use(cors.Default())

	//HTTP for boot only
	efihttp := gin.New()
	efihttp.Use(cors.Default())

	uiFS, err := webui.FS()
	if err != nil {
		logrus.Fatal(err)
	}
	uiServer := http.FileServer(http.FS(uiFS))

	// ks.cfg is served at top to not place it behind BasicAuth
	https.GET("ks.cfg", api.Ks(key))

	// middleware to log static file requests over https
	https.Use(func(c *gin.Context) {
		if c.Request.URL.Path == "/esx.iso" {
			logrus.WithFields(logrus.Fields{
				"path":   c.Request.URL.Path,
				"method": c.Request.Method,
				"ip":     c.ClientIP(),
			}).Info("static_file_request")
		}
		c.Next()
	})

	// middleware to log static file requests over http
	efihttp.Use(func(c *gin.Context) {
		if c.Request.URL.Path == "/esx.iso" {
			logrus.WithFields(logrus.Fields{
				"path":   c.Request.URL.Path,
				"method": c.Request.Method,
				"ip":     c.ClientIP(),
			}).Info("static_file_request")
		}
		c.Next()
	})

	// uefi-https boot
	https.GET("/esx/*filepath", uefi.Files(conf))

	// uefi-http boot
	efihttp.GET("/esx/*filepath", uefi.Files(conf))

	// middleware to check if user is logged in
	https.Use(func(c *gin.Context) {

		// Check for basic auth
		username, password, hasAuth := c.Request.BasicAuth()
		if !hasAuth {
			logrus.WithFields(logrus.Fields{
				"login": "unauthorized request",
			}).Info("auth")
			//c.Writer.Header().Set("WWW-Authenticate", "Basic realm=Restricted")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		//get the user that is trying to authenticate
		var user models.User
		if res := db.DB.Select("username", "password").Where("username = ?", username).First(&user); res.Error != nil {
			logrus.WithFields(logrus.Fields{
				"username": username,
				"status":   "supplied username does not exist",
			}).Info("auth")
			//c.Writer.Header().Set("WWW-Authenticate", "Basic realm=Restricted")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		//check if passwords match
		if api.ComparePasswords(user.Password, []byte(password), username) {
			logrus.WithFields(logrus.Fields{
				"username": username,
				"status":   "successfully authenticated",
			}).Debug("auth")
		} else {
			logrus.WithFields(logrus.Fields{
				"username": username,
				"status":   "invalid password supplied",
			}).Info("auth")
			//c.Writer.Header().Set("WWW-Authenticate", "Basic realm=Restricted")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Next()
	})

	https.NoRoute(func(c *gin.Context) {
		c.Request.URL.Path = "/" // always return index.html rather than the requested page, to be compatible with HTML5 routing
		uiServer.ServeHTTP(c.Writer, c.Request)
	})

	ui := https.Group("/")
	{
		ui.GET("/web/*all", gin.WrapH(http.StripPrefix("/web", uiServer)))
		ui.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	v1 := https.Group("/v1")
	{

		hosts := v1.Group("/hosts")
		{
			hosts.GET("", api.ListHosts)
			hosts.GET(":id", api.GetHost)
			hosts.POST("/search", api.SearchHost)
			hosts.POST("", api.CreateHost)
			hosts.PATCH(":id", api.UpdateHost)
			hosts.DELETE(":id", api.DeleteHost)
		}

		deviceClass := v1.Group("/device_classes")
		{
			deviceClass.GET("", api.ListDeviceClasses)
			deviceClass.GET(":id", api.GetDeviceClass)
			deviceClass.POST("/search", api.SearchDeviceClass)
			deviceClass.POST("", api.CreateDeviceClass)
			deviceClass.PATCH(":id", api.UpdateDeviceClass)
			deviceClass.DELETE(":id", api.DeleteDeviceClass)
		}

		groups := v1.Group("/groups")
		{
			groups.GET("", api.ListGroups)
			groups.GET(":id", api.GetGroup)
			groups.POST("", api.CreateGroup(key))
			groups.PATCH(":id", api.UpdateGroup(key))
			groups.DELETE(":id", api.DeleteGroup)
		}

		images := v1.Group("/images")
		{
			images.GET("", api.ListImages)
			images.GET(":id", api.GetImage)
			images.POST("", api.CreateImage)
			images.PATCH(":id", api.UpdateImage)
			images.DELETE(":id", api.DeleteImage)
		}

		users := v1.Group("/users")
		{
			users.GET("", api.ListUsers)
			users.GET(":id", api.GetUser)
			users.POST("", api.CreateUser)
			users.PATCH(":id", api.UpdateUser)
			users.DELETE(":id", api.DeleteUser)
		}

		postconfig := v1.Group("/postconfig")
		{
			postconfig.GET("", api.PostConfig(key))
			postconfig.GET(":id", api.PostConfigID(key))
		}

		v1.GET("log", logServer.Handle)

		v1.GET("version", api.Version(version, commit, date))
	}

	/*	r.GET("postconfig", api.PostConfig) */

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
