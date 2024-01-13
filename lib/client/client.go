package client

import (
	"github.com/teknofire/question-queue/models"
	"gorm.io/gorm"
)

type Client struct {
	DB *gorm.DB
}

func (c Client) Pop(name string) (models.Question, bool) {
	q := models.Question{Queue: name}

	result := c.DB.Where(models.Question{Queue: name}).
		Order("created_at asc").
		First(&q)

	if result.Error != nil {
		return q, false
	}
	c.DB.Delete(&q)

	return q, true
}

func (c Client) All(name string) []models.Question {
	questions := []models.Question{}

	c.DB.Where(models.Question{Queue: name}).
		Order("created_at asc").
		Find(&questions)

	return questions
}

func (c Client) Count(name string) int64 {
	q := models.Question{Queue: name}

	var count int64
	c.DB.Where(&q).Count(&count)

	return count
}
