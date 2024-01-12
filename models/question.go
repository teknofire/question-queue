package models

import (
	"gorm.io/gorm"
)

type Question struct {
	gorm.Model

	Queue string `gorm:"index:idx_name"`
	Text  string `form:"q"`
}

func (q Question) String() string {
	return q.Text
}
