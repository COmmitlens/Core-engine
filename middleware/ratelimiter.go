package middleware

import (
	"context"
	"fmt"
	"net/http"

	"github.com/labstack/echo"
	"github.com/redis/go-redis/v9"
)

// The "Secret Sauce" Script
var requestRateScript = redis.NewScript(`
    local key = KEYS[1]
    local limit = tonumber(ARGV[1])
    local window = tonumber(ARGV[2])

    -- Increment the counter
    local current = redis.call("INCR", key)

    -- If it's the first request, set the expiry
    if current == 1 then
        redis.call("EXPIRE", key, window)
    end

    -- Return the current count
    return current
`)

func RateLimitMiddleware(rdb *redis.Client, limit, window int, keyPrefix string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ip := c.RealIP()
			key := fmt.Sprintf("rate_limit:%s:%s", keyPrefix, ip)

			// EXECUTE LUA SCRIPT (Atomic!)
			count, err := requestRateScript.Run(context.Background(), rdb, []string{key}, limit, window).Int()

			if err != nil {
				// Fail open (allow request) if Redis is down, but log it
				return next(c)
			}

			// Check Limit
			if count > limit {
				return c.JSON(http.StatusTooManyRequests, map[string]string{
					"error": "Rate limit exceeded.",
				})
			}

			return next(c)
		}
	}
}
