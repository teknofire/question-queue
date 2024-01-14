package custom_middleware

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type (
	ApiKey struct {
		Key    string
		Header bool
		Config ApiKeyConfig
	}

	ApiKeyConfig struct {
		Skipper middleware.Skipper

		QueryKeyName  string
		HeaderKeyName string
	}
)

var (
	DefaultApiKeyConfig = ApiKeyConfig{
		Skipper:       middleware.DefaultSkipper,
		QueryKeyName:  "key",
		HeaderKeyName: "API-KEY",
	}
)

func NewApiKey() *ApiKey {
	return NewApiKeyWithConfig(DefaultApiKeyConfig)
}

func NewApiKeyWithConfig(config ApiKeyConfig) *ApiKey {
	return &ApiKey{
		Key:    "",
		Header: false,
		Config: config,
	}
}

func (a *ApiKey) Handler(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		a.Key = ""
		a.Header = false

		param := c.QueryParam(a.Config.QueryKeyName)
		head := c.Request().Header.Get(a.Config.HeaderKeyName)

		if len(param) > 0 {
			a.Key = param
			a.Header = false
		}

		if len(head) > 0 {
			a.Key = head
			a.Header = true
		}

		return next(c)
	}
}
