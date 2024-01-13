package client

import (
	"github.com/labstack/gommon/log"
	"github.com/teknofire/question-queue/models"
	"gorm.io/gorm"
)

type Client struct {
	DB *gorm.DB
}

func (c Client) Pop(name string) (models.Question, bool) {
	q := models.Question{Queue: name}

	result := c.DB.Order("created_at asc").First(&q)
	log.Infof("%+v", q.ID)
	if result.Error != nil {
		return q, false
	}
	c.DB.Delete(&q)

	return q, true
}

func (c Client) All(name string) []models.Question {
	questions := []models.Question{}

	q := models.Question{Queue: name}

	c.DB.Where(&q).Order("created_at asc").Find(&questions)

	return questions
}

func (c Client) Count(name string) int64 {
	q := models.Question{Queue: name}

	var count int64
	c.DB.Where(&q).Count(&count)

	return count
}
