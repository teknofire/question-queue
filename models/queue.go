package models

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"github.com/labstack/gommon/log"
	"gorm.io/gorm"
)

type Queue struct {
	gorm.Model

	Name   string
	ApiKey string
}

func (q *Queue) GenerateApiKey(name string) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		log.Error(err)
		return
	}

	key := fmt.Sprintf("%b%b", []byte(name), b)

	hash := sha256.Sum256([]byte(key))
	q.ApiKey = base64.URLEncoding.EncodeToString(hash[:])
}
