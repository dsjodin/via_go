package server

import (
	"net/http"
	"time"

	"github.com/dsjodin/via_go/internal/api"
	"github.com/dsjodin/via_go/internal/auth"
	"github.com/dsjodin/via_go/internal/config"
	"github.com/dsjodin/via_go/internal/websockets"
	"github.com/dsjodin/via_go/webui"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// Version identifies the running build, for the /v1/version endpoint.
type Version struct {
	Version string
	Commit  string
	Date    string
}

// Options are the dependencies the routers need from main.
type Options struct {
	Config *config.Config

	// SecretKey decrypts stored ESXi passwords.
	SecretKey string

	// LogServer streams the log to the UI over a websocket.
	LogServer *websockets.LogServer

	// Auth holds the session store and login throttle. NewRouters creates one
	// if it is nil.
	Auth *api.Auth

	Version Version
}

// NewRouters builds the two HTTP engines go-via serves.
//
// They are separate because one of them is unauthenticated by necessity: a
// host doing UEFI HTTP boot has no credentials, so the boot files are served
// over plain HTTP on :80, while the API and UI stay behind TLS and
// authentication on the configured port.
func NewRouters(opts Options) (apiRouter *gin.Engine, bootRouter *gin.Engine, err error) {
	if opts.Auth == nil {
		opts.Auth = &api.Auth{
			Sessions: auth.NewSessions(auth.DefaultTTL),
			Throttle: auth.NewThrottle(10, 15*time.Minute),
			// The API listens on TLS, so the session cookie is TLS only.
			Secure: true,
		}
	}

	uiFS, err := webui.FS()
	if err != nil {
		return nil, nil, err
	}

	return newAPIRouter(opts, http.FileServer(http.FS(uiFS))), newBootRouter(opts), nil
}

// newBootRouter serves only the ESXi boot files, unauthenticated, for clients
// that have no way to authenticate.
func newBootRouter(opts Options) *gin.Engine {
	r := gin.New()
	r.Use(cors.Default())
	r.Use(logStaticFileRequests())
	r.GET("/esx/*filepath", Files(opts.Config))

	return r
}

func newAPIRouter(opts Options, uiServer http.Handler) *gin.Engine {
	r := gin.New()
	r.Use(cors.Default())

	// ks.cfg is registered before the auth middleware: the installer fetching
	// it has no credentials either. It is authorised by source address.
	r.GET("ks.cfg", api.Ks(opts.SecretKey))

	// Logging in cannot itself require being logged in.
	r.POST("/v1/login", opts.Auth.Login)

	// A freshly installed host reports completion here at the end of its
	// kickstart. It has no credentials, so like ks.cfg this sits in front of
	// the auth middleware and is authorised by source address against a host
	// record instead. The by-id form below stays behind auth: that one is an
	// operator re-running post-config by hand.
	r.GET("/v1/postconfig", api.PostConfig(opts.SecretKey))

	r.Use(logStaticFileRequests())

	// The same boot files are also reachable over HTTPS, for hosts configured
	// to boot that way.
	r.GET("/esx/*filepath", Files(opts.Config))

	r.Use(opts.Auth.Middleware())

	r.NoRoute(func(c *gin.Context) {
		// Always return index.html rather than the requested path, so the
		// single page app can handle its own routing.
		c.Request.URL.Path = "/"
		uiServer.ServeHTTP(c.Writer, c.Request)
	})

	ui := r.Group("/")
	{
		ui.GET("/web/*all", gin.WrapH(http.StripPrefix("/web", uiServer)))
		ui.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	registerV1(r, opts)

	return r
}

func registerV1(r *gin.Engine, opts Options) {
	v1 := r.Group("/v1")

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
		groups.POST("", api.CreateGroup(opts.SecretKey))
		groups.PATCH(":id", api.UpdateGroup(opts.SecretKey))
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
		users.POST("", opts.Auth.CreateUser)
		users.PATCH(":id", opts.Auth.UpdateUser)
		users.DELETE(":id", api.DeleteUser)
	}

	v1.GET("postconfig/:id", api.PostConfigID(opts.SecretKey))

	if opts.LogServer != nil {
		v1.GET("log", opts.LogServer.Handle)
	}

	v1.POST("logout", opts.Auth.Logout)
	v1.GET("session", opts.Auth.Session)

	v1.GET("version", api.Version(opts.Version.Version, opts.Version.Commit, opts.Version.Date))
}

func logStaticFileRequests() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/esx.iso" {
			logrus.WithFields(logrus.Fields{
				"path":   c.Request.URL.Path,
				"method": c.Request.Method,
				"ip":     c.ClientIP(),
			}).Info("static_file_request")
		}
		c.Next()
	}
}
