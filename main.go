package main

import (
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/sirupsen/logrus"
	"github.com/xiaoowen/chatgpt-proxy/apis"
	"github.com/xiaoowen/chatgpt-proxy/data"
	"log"
	"net/http"
	"path"
	"runtime"
)

func init() {
	initFns := []func(){
		initLog, data.InitRedis,
	}
	for _, fn := range initFns {
		fn()
	}
	initFnsReturnErr := []func() error{
		data.InitUsers, data.InitGPTSetting,
	}
	for _, fn := range initFnsReturnErr {
		if err := fn(); err != nil {
			logrus.Fatalf("init failed. %s", err.Error())
		}
	}
}

func initLog() {
	logrus.SetLevel(logrus.InfoLevel)
	logrus.SetReportCaller(true)
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		CallerPrettyfier: func(frame *runtime.Frame) (function string, file string) {
			return "", fmt.Sprintf(" %s:%d", path.Base(frame.File), frame.Line)
		},
	})
}

type router struct {
	Method  string
	Path    string
	Handler func(ctx *fiber.Ctx) error
}

var routers = []router{
	{Method: http.MethodGet, Path: "/hello", Handler: apis.Hello},
	{Method: http.MethodGet, Path: "/models", Handler: apis.Models},
	{Method: http.MethodPost, Path: "/newuser", Handler: apis.NewUser},
	{Method: http.MethodPost, Path: "/edituser", Handler: apis.EditUser},
	{Method: http.MethodGet, Path: "/users", Handler: apis.Users},
	{Method: http.MethodGet, Path: "/profile", Handler: apis.Profile},
	{Method: http.MethodGet, Path: "/setting", Handler: apis.Setting},
	{Method: http.MethodPost, Path: "/settingedit", Handler: apis.EditSetting},
	{Method: http.MethodPost, Path: "/chat", Handler: apis.Chat},
	{Method: http.MethodGet, Path: "/chatstream", Handler: apis.StreamChat},
}

func main() {
	app := fiber.New()
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "*",
		AllowHeaders:     "content-type,access-token",
		AllowCredentials: true,
		AllowMethods:     "GET,POST",
	}))

	for _, item := range routers {
		if item.Method == http.MethodGet {
			app.Get(item.Path, item.Handler)
		} else if item.Method == http.MethodPost {
			app.Post(item.Path, item.Handler)
		}
	}

	if err := app.Listen(":80"); err != nil {
		log.Fatalln(err)
	}
}
