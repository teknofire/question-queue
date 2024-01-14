package main

import (
	"crypto/subtle"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"github.com/teknofire/question-queue/lib/client"
	custom_middleware "github.com/teknofire/question-queue/middleware"
	"github.com/teknofire/question-queue/model"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite" // Sqlite driver based on CGO
	"gorm.io/gorm"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type QuestionList []model.Question

func (ql QuestionList) FindIndex(id uint) int {
	for i, q := range ql {
		if q.ID == id {
			return i
		}
	}

	return -1
}

type Template struct {
	templates *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	if viewContext, isMap := data.(map[string]interface{}); isMap {
		viewContext["key"] = c.QueryParam("key")
	}

	return t.templates.ExecuteTemplate(w, name, data)
}

func main() {
	app := client.Client{
		ApiKey: custom_middleware.NewApiKey(),
	}

	port := os.Getenv("PORT")
	if port == "" {
		log.Fatal("$PORT must be set")
	}

	database_url := os.Getenv("DATABASE_URL")
	if database_url == "" {
		db, err := gorm.Open(sqlite.Open("questions.sqlite"), &gorm.Config{})
		if err != nil {
			log.Fatal(err)
		}
		app.DB = db
	} else {
		db, err := gorm.Open(postgres.Open(database_url), &gorm.Config{})
		if err != nil {
			log.Fatal(err)
		}
		app.DB = db
	}

	app.DB.AutoMigrate(&model.Question{}, &model.Queue{})

	e := echo.New()

	apiKey := custom_middleware.NewApiKey()
	app.ApiKey = apiKey

	e.Pre(app.ApiKey.Handler)

	e.Pre(middleware.MethodOverrideWithConfig(middleware.MethodOverrideConfig{
		Getter: middleware.MethodFromForm("_method"),
	}))
	e.Use(middleware.RequestID())

	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogURI:         true,
		LogStatus:      true,
		LogError:       true,
		LogMethod:      true,
		LogHost:        true,
		LogLatency:     true,
		LogUserAgent:   true,
		LogHeaders:     []string{"API-KEY"},
		LogQueryParams: []string{"key"},
		HandleError:    true, // forwards error to the global error handler, so it can decide appropriate status code
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			if v.Error == nil {
				log.WithFields(logrus.Fields{
					"URI":       v.URI,
					"status":    v.Status,
					"method":    v.Method,
					"host":      v.Host,
					"key":       v.QueryParams["key"],
					"api-key":   v.Headers["API-KEY"],
					"latency":   v.Latency,
					"useragent": v.UserAgent,
				}).Info("request")
			} else {
				log.WithFields(logrus.Fields{
					"URI":       v.URI,
					"status":    v.Status,
					"method":    v.Method,
					"host":      v.Host,
					"key":       v.QueryParams["key"],
					"api-key":   v.Headers["API-KEY"],
					"latency":   v.Latency,
					"useragent": v.UserAgent,
					"error":     v.Error,
				}).Error("request error")
			}
			return nil
		},
	}))

	allowedPaths := []string{
		"/public", "/setup", "/favicon.ico", "/register",
	}
	e.Use(middleware.KeyAuthWithConfig(middleware.KeyAuthConfig{
		Skipper: func(c echo.Context) bool {
			if c.Path() == "/" {
				return true
			}

			for _, path := range allowedPaths {
				if strings.HasPrefix(c.Request().URL.Path, path) {
					return true
				}
			}
			return false
		},
		KeyLookup: "header:API-KEY,query:key",
		Validator: func(key string, c echo.Context) (bool, error) {
			queue := c.Param("queue")

			var q model.Queue
			result := app.DB.Where("name = ?", queue).First(&q)

			if result.RowsAffected == 0 {
				return false, fmt.Errorf("%s not found", queue)
			}

			return subtle.ConstantTimeCompare([]byte(key), []byte(q.ApiKey)) == 1, nil
		},
	}))

	e.Use(middleware.RecoverWithConfig(middleware.RecoverConfig{
		StackSize: 1 << 10, // 1 KB
	}))

	funcs := template.FuncMap{
		"questionUrl": app.QuestionUrl,
	}

	templates := &Template{
		templates: template.Must(template.New("root").Funcs(funcs).ParseGlob("public/views/*.html")),
	}

	e.Renderer = templates
	e.Static("/public/css", "public/css")
	e.Static("/public/js", "bower_components/")
	e.Static("/favicon.ico", "public/favicon.ico")

	e.GET("/", func(c echo.Context) error {
		var q model.Queue
		if len(app.ApiKey.Key) > 0 {
			app.DB.Where("api_key = ?", app.ApiKey.Key).First(&q)
			return c.Redirect(http.StatusTemporaryRedirect, app.QueueUrl(q.Name))
		}

		return c.String(http.StatusOK, "Welcome")
	})

	e.GET("/:queue", func(c echo.Context) error {
		queue := c.Param("queue")
		key := c.QueryParam("key")

		return c.Render(http.StatusOK, "dashboard.html", map[string]interface{}{
			"Queue":     queue,
			"Key":       key,
			"Questions": app.All(queue),
		})
	})

	e.POST("/:queue", func(c echo.Context) error {
		question := model.Question{}

		question.Queue = c.Param("queue")
		question.Text = c.FormValue("q")

		app.DB.Create(&question)

		return c.String(http.StatusOK, question.String())
	})

	e.GET("/:queue/all", func(c echo.Context) error {
		queue := c.Param("queue")

		questions := app.All(queue)

		output := []string{}
		for _, q := range questions {
			output = append(output, fmt.Sprintf("%d: %s\n", q.ID, q.String()))
		}

		return c.String(http.StatusOK, strings.Join(output, ""))
	})

	e.GET("/:queue/count", func(c echo.Context) error {
		queue := c.Param("queue")

		questions := app.All(queue)

		return c.String(http.StatusOK, fmt.Sprintf("%d", len(questions)))
	})

	e.GET("/:queue/pop", func(c echo.Context) error {
		queue := c.Param("queue")

		if q, ok := app.Pop(queue); ok {
			return c.String(http.StatusOK, q.String())
		}

		return c.String(http.StatusBadRequest, "No questions in the queue\n")
	})

	e.DELETE("/:queue/:id", func(c echo.Context) error {
		queue := c.Param("queue")
		id := c.Param("id")

		app.DB.Delete(&model.Question{}, id)

		return c.Redirect(http.StatusTemporaryRedirect, app.QueueUrl(queue))
	})

	e.GET("/:queue/overlay", func(c echo.Context) error {
		queue := c.Param("queue")

		questions := app.All(queue)

		return c.Render(http.StatusOK, "overlay.html", len(questions))
	})

	e.POST("/register/:name", func(c echo.Context) error {
		name := c.Param("name")

		var q model.Queue
		result := app.DB.Where("name = ?", name).First(&q)
		if result.RowsAffected > 0 {
			return c.String(http.StatusBadRequest, "Name already exists")
		}

		q.GenerateApiKey(name)
		app.DB.Create(&q)
		return c.String(http.StatusOK, q.ApiKey)
	})

	e.Logger.Fatal(e.Start(":" + port))
}
