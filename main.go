package main

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/teknofire/question-queue/lib/client"
	"github.com/teknofire/question-queue/models"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite" // Sqlite driver based on CGO
	"gorm.io/gorm"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type QuestionList []models.Question

var (
	ApiKey = ""
)

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
	app := client.Client{}

	ApiKey := os.Getenv("API_KEY")

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

	app.DB.AutoMigrate(&models.Question{})

	funcs := template.FuncMap{
		"url": func(q models.Question, path ...string) string {
			uri := []string{fmt.Sprintf("/%s/%d", q.Queue, q.ID)}
			uri = append(uri, path...)

			return fmt.Sprintf("%s?key=%s", strings.Join(uri, "/"), ApiKey)
		},
	}

	templates := &Template{
		templates: template.Must(template.New("root").Funcs(funcs).ParseGlob("public/views/*.html")),
	}

	e := echo.New()

	e.Pre(middleware.MethodOverrideWithConfig(middleware.MethodOverrideConfig{
		Getter: middleware.MethodFromForm("_method"),
	}))
	e.Renderer = templates
	e.Use(middleware.RequestID())
	e.Use(middleware.Logger())

	allowedPaths := []string{
		"/public", "/setup", "/favicon",
	}
	e.Use(middleware.KeyAuthWithConfig(middleware.KeyAuthConfig{
		Skipper: func(c echo.Context) bool {
			for _, p := range allowedPaths {
				if strings.HasPrefix(c.Path(), p) {
					return true
				}
			}
			return false
		},
		KeyLookup: "query:key",
		Validator: func(key string, c echo.Context) (bool, error) {
			return key == ApiKey, nil
		},
	}))

	e.Use(middleware.RecoverWithConfig(middleware.RecoverConfig{
		StackSize: 1 << 10, // 1 KB
	}))

	e.Static("/public/css", "public/css")
	e.Static("/public/js", "bower_components/")

	e.GET("/:name", func(c echo.Context) error {
		name := c.Param("name")
		key := c.QueryParam("key")

		return c.Render(http.StatusOK, "dashboard.html", map[string]interface{}{
			"Queue":     name,
			"Key":       key,
			"Questions": app.All(name),
		})
	})

	e.POST("/:name", func(c echo.Context) error {
		question := models.Question{}

		question.Queue = c.Param("name")
		question.Text = c.FormValue("q")

		app.DB.Create(&question)

		return c.String(http.StatusOK, question.String())
	})

	e.GET("/:name/all", func(c echo.Context) error {
		name := c.Param("name")

		queue := app.All(name)

		output := []string{}
		for _, q := range queue {
			output = append(output, fmt.Sprintf("%d: %s\n", q.ID, q.String()))
		}

		return c.String(http.StatusOK, strings.Join(output, ""))
	})

	e.GET("/:name/count", func(c echo.Context) error {
		name := c.Param("name")

		queue := app.All(name)

		return c.String(http.StatusOK, fmt.Sprintf("%d", len(queue)))
	})

	e.GET("/:name/pop", func(c echo.Context) error {
		name := c.Param("name")

		if q, ok := app.Pop(name); ok {
			return c.String(http.StatusOK, q.String())
		}

		return c.String(http.StatusBadRequest, "No questions in the queue\n")
	})

	e.DELETE("/:name/:id", func(c echo.Context) error {
		name := c.Param("name")
		id := c.Param("id")

		app.DB.Delete(&models.Question{}, id)

		return c.Redirect(http.StatusMovedPermanently, fmt.Sprintf("/%s?key=%s", name, ApiKey))
	})

	e.GET("/:name/overlay", func(c echo.Context) error {
		name := c.Param("name")
		count := app.Count(name)

		return c.Render(http.StatusOK, "overlay.html", count)
	})

	e.Logger.Fatal(e.Start(":" + port))
}
