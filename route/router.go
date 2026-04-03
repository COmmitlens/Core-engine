package route

import (
	"core/config"
	"net/http"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/redis/go-redis/v9"
)

func InitHttp() *echo.Echo {
	app := App()
	cfg := config.GetConfig()
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
	})
	e := echo.New()
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     []string{"https://book-finder0908sid.netlify.app", "http://localhost:3000", "https://commitlens.tech"}, // Add your frontend URLs
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete},
		AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
		AllowCredentials: true,
	}))
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "OfficeMesh GitHub App is running")
	})

	v1Routes(e.Group("/v1"), app, rdb)
	return e
}
