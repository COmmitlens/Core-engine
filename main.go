package main

import (
	"core/config"
	"core/route"
	"log"
	"net/http"

	"github.com/joho/godotenv"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file")
	}

	// Middleware
	config.DbInit()
	e := route.InitHttp()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.GET("/health1", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	// Start server
	e.Logger.Fatal(e.Start(":8000"))
}
