package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type Webhook struct {
	ID             int
	Name           string
	Url            string
	IsGlobal       bool
	Conn           *string
	SendMessage    bool
	ReceiveMessage bool
}

type WebhookMessage struct {
	Conn       string `json:"conn"`
	Message    string `json:"message"`
	SentBy     string `json:"sent_by"`
	Department string `json:"department"`
	Agent      string `json:"agent"`
	Tag        string `json:"tag"`
	IsOpen     bool   `json:"is_open"`
}

func SendWebhook(url string, msg *WebhookMessage) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %s", resp.Status)
	}
	return nil
}
