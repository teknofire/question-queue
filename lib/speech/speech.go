package speech

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/teknofire/question-queue/model"
)

type urlFunc func(string) string

type Voice string

const (
	ADAM Voice = "pNInz6obpgDQGcFmaJgB"
)

type Model string

const (
	ENGLISH_V1 Model = "eleven_monolingual_v1"
)

type speech struct {
	apiKey   string
	endpoint string
	model    Model
	settings VoiceSettings
}

type Data struct {
	ModelId       Model         `json:"model_id"`
	Text          string        `json:"text"`
	VoiceSettings VoiceSettings `json:"voice_settings"`
}

type VoiceSettings struct {
	SimilarityBoost float32 `json:"similarity_boost"`
	Stability       float32 `json:"stability"`
	Style           int     `json:"style"`
	SpeakerBoost    bool    `json:"use_speaker_boost"`
}

func New(apiKey string, voice Voice, model Model) *speech {
	return &speech{
		apiKey:   apiKey,
		model:    model,
		endpoint: fmt.Sprintf("https://api.elevenlabs.io/v1/text-to-speech/%s", voice),
		settings: VoiceSettings{
			SimilarityBoost: 0.75,
			Stability:       0.5,
			Style:           0,
			SpeakerBoost:    true,
		},
	}
}

func (s speech) createRequest(q *model.Question) (*http.Request, error) {

	data := Data{
		ModelId:       s.model,
		Text:          q.Text,
		VoiceSettings: s.settings,
	}
	ttsData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	payload := strings.NewReader(string(ttsData))

	req, _ := http.NewRequest("POST", s.endpoint, payload)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "audio/mpeg")
	req.Header.Add("xi-api-key", s.apiKey)

	return req, nil
}

func (s speech) Stream(q *model.Question, f io.Writer) error {
	req, err := s.createRequest(q)
	if err != nil {
		return err
	}

	res, _ := http.DefaultClient.Do(req)

	defer res.Body.Close()

	_, err = io.Copy(f, res.Body)

	return err
}

func (s speech) CreateFile(q *model.Question) (string, error) {
	req, err := s.createRequest(q)
	if err != nil {
		return "", err
	}

	res, _ := http.DefaultClient.Do(req)

	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)

	filename := fmt.Sprintf("%s/%s_%d.mp3", "audio", q.Queue, q.ID)
	fh, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer fh.Close()

	fh.Write(body)
	fh.Sync()

	return filename, nil
}
