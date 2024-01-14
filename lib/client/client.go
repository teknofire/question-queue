package client

import (
	"fmt"
	"strings"

	custom_middleware "github.com/teknofire/question-queue/middleware"
	"github.com/teknofire/question-queue/model"
	"gorm.io/gorm"
)

type Client struct {
	ApiKey *custom_middleware.ApiKey
	DB     *gorm.DB
}

func (c *Client) QueueUrl(queue string, path ...string) string {
	parts := []string{queue}
	uri := "/" + strings.Join(append(parts, path...), "/")

	return c.AppendApiKey(uri)
}

func (c *Client) QuestionUrl(q model.Question, path ...string) string {
	parts := []string{q.Queue, fmt.Sprintf("%d", q.ID)}
	uri := "/" + strings.Join(append(parts, path...), "/")

	return c.AppendApiKey(uri)
}

func (c *Client) AppendApiKey(uri string) string {
	if !c.ApiKey.Header && len(c.ApiKey.Key) > 0 {
		uri = fmt.Sprintf("%s?key=%s", uri, c.ApiKey.Key)
	}

	return uri
}

func (c Client) Pop(name string) (model.Question, bool) {
	q := model.Question{Queue: name}

	result := c.DB.Where(model.Question{Queue: name}).
		Order("created_at asc").
		First(&q)

	if result.Error != nil {
		return q, false
	}
	c.DB.Delete(&q)

	return q, true
}

func (c Client) All(name string) []model.Question {
	questions := []model.Question{}

	c.DB.Where(model.Question{Queue: name}).
		Order("created_at asc").
		Find(&questions)

	return questions
}

func (c Client) Count(name string) int64 {
	q := model.Question{Queue: name}

	var count int64
	c.DB.Where(&q).Count(&count)

	return count
}
