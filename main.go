package main

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/teknofire/question-queue/models"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite" // Sqlite driver based on CGO
	"gorm.io/gorm"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type Queues struct {
	DB *gorm.DB
}

type QuestionList []models.Question

var (
	ApiKey = "cheesetoast"
)

func (ql QuestionList) FindIndex(id uint) int {
	for i, q := range ql {
		if q.ID == id {
			return i
		}
	}

	return -1
}

func (qs Queues) Pop(name string) (models.Question, bool) {
	q := models.Question{Queue: name}

	result := qs.DB.Order("created_at asc").First(&q)
	log.Infof("%+v", q.ID)
	if result.Error != nil {
		return q, false
	}
	qs.DB.Delete(&q)

	return q, true
}

func (qs Queues) All(name string) QuestionList {
	questions := []models.Question{}

	q := models.Question{Queue: name}

	qs.DB.Where(&q).Order("created_at asc").Find(&questions)

	return questions
}

func (qs Queues) Count(name string) int64 {
	q := models.Question{Queue: name}

	var count int64
	qs.DB.Where(&q).Count(&count)

	return count
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
	queues := Queues{}

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
		queues.DB = db
	} else {
		db, err := gorm.Open(postgres.Open(database_url), &gorm.Config{})
		if err != nil {
			log.Fatal(err)
		}
		queues.DB = db
	}

	queues.DB.AutoMigrate(&models.Question{})

	funcs := template.FuncMap{
		"url": func(q models.Question, path ...string) string {
			uri := []string{fmt.Sprintf("/queue/%s/%d", q.Queue, q.ID)}
			uri = append(uri, path...)

			return fmt.Sprintf("%s?key=%s", strings.Join(uri, "/"), ApiKey)
		},
	}

	templates := &Template{
		templates: template.Must(template.New("root").Funcs(funcs).ParseGlob("public/views/*.html")),
	}

	e := echo.New()
	e.Renderer = templates
	e.Use(middleware.RequestID())
	e.Use(middleware.Logger())
	e.Use(middleware.KeyAuthWithConfig(middleware.KeyAuthConfig{
		Skipper: func(c echo.Context) bool {
			log.Printf("%s", c.Path())
			return strings.HasPrefix(c.Path(), "/public")
		},
		KeyLookup: "query:key",
		Validator: func(key string, c echo.Context) (bool, error) {
			return key == ApiKey, nil
		},
	}))

	e.Static("/public/css", "public/css")
	e.Static("/public/js", "bower_components/")

	e.GET("/dashboard/:name", func(c echo.Context) error {
		name := c.Param("name")
		key := c.QueryParam("key")

		return c.Render(http.StatusOK, "dashboard.html", map[string]interface{}{
			"Queue":     name,
			"Key":       key,
			"Questions": queues.All(name),
		})
	})

	e.GET("/queue/:name/all", func(c echo.Context) error {
		name := c.Param("name")

		queue := queues.All(name)

		output := []string{}
		for _, q := range queue {
			output = append(output, fmt.Sprintf("%d: %s\n", q.ID, q.String()))
		}

		return c.String(http.StatusOK, strings.Join(output, ""))
	})

	e.GET("/queue/:name/count", func(c echo.Context) error {
		name := c.Param("name")

		queue := queues.All(name)

		return c.String(http.StatusOK, fmt.Sprintf("%d", len(queue)))
	})

	e.GET("/queue/:name/pop", func(c echo.Context) error {
		name := c.Param("name")

		if q, ok := queues.Pop(name); ok {
			return c.String(http.StatusOK, q.String())
		}

		return c.String(http.StatusBadRequest, "No questions in the queue\n")
	})

	e.POST("/queue/:name/:id/delete", func(c echo.Context) error {
		name := c.Param("name")
		id := c.Param("id")

		queues.DB.Delete(&models.Question{}, id)

		return c.Redirect(http.StatusMovedPermanently, fmt.Sprintf("/dashboard/%s?key=%s", name, ApiKey))
	})

	e.POST("/queue/:name", func(c echo.Context) error {
		question := models.Question{}

		question.Queue = c.Param("name")
		question.Text = c.FormValue("q")

		queues.DB.Create(&question)

		return c.String(http.StatusOK, question.String())
	})

	e.GET("/overlay/:name", func(c echo.Context) error {
		name := c.Param("name")
		count := queues.Count(name)

		return c.Render(http.StatusOK, "overlay.html", count)
	})

	e.Logger.Fatal(e.Start(":" + port))
}
