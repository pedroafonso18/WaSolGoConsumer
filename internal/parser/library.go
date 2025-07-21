package parser

import (
	"encoding/json"
)

type Request struct {
	Action  string            `json:"action"`
	Method  string            `json:"method"`
	Url     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    map[string]string `json:"body,omitempty"`
	Params  map[string]string `json:"params,omitempty"`
}

type Chat struct {
	ID         string  `json:"id"`
	Situation  string  `json:"situation"`
	IsActive   bool    `json:"is_active"`
	AgentID    string  `json:"agent_id"`
	Tabulation *string `json:"tabulation,omitempty"`
	CustomerID string  `json:"customer_id"`
	InstanceID string  `json:"instance_id"`
}

type Message struct {
	ID        int    `json:"id"`
	From      string `json:"from"`
	To        string `json:"to"`
	Delivered bool   `json:"delivered"`
	Text      string `json:"text"`
	ChatID    string `json:"chat_id"`
}

type Customer struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Number     string  `json:"number"`
	LastChatID *string `json:"last_chat_id,omitempty"`
}

type RabbitResponse struct {
	Webhook WebhookMessage `json:"webhook"`
	ChatID  string         `json:"chat_id"`
}

type WebhookMessage struct {
	Headers       WebhookHeaders    `json:"headers"`
	Params        map[string]string `json:"params"`
	Query         map[string]string `json:"query"`
	Body          WebhookBody       `json:"body"`
	WebhookUrl    string            `json:"webhookUrl"`
	ExecutionMode string            `json:"executionMode"`
}

type WebhookHeaders struct {
	Host             string `json:"host"`
	UserAgent        string `json:"user-agent"`
	ContentLength    string `json:"content-length"`
	AcceptEncoding   string `json:"accept-encoding"`
	ContentType      string `json:"content-type"`
	XForwardedFor    string `json:"x-forwarded-for"`
	XForwardedHost   string `json:"x-forwarded-host"`
	XForwardedPort   string `json:"x-forwarded-port"`
	XForwardedProto  string `json:"x-forwarded-proto"`
	XForwardedServer string `json:"x-forwarded-server"`
	XRealIP          string `json:"x-real-ip"`
}

type WebhookBody struct {
	Event       string      `json:"event"`
	Instance    string      `json:"instance"`
	Data        WebhookData `json:"data"`
	Destination string      `json:"destination"`
	DateTime    string      `json:"date_time"`
	Sender      string      `json:"sender"`
	ServerUrl   string      `json:"server_url"`
	APIKey      string      `json:"apikey"`
}

type WebhookData struct {
	Key              MessageKey     `json:"key"`
	PushName         string         `json:"pushName"`
	Message          MessageContent `json:"message"`
	MessageType      string         `json:"messageType"`
	MessageTimestamp int64          `json:"messageTimestamp"`
	InstanceID       string         `json:"instanceId"`
	Source           string         `json:"source"`
}

type MessageKey struct {
	RemoteJid string `json:"remoteJid"`
	FromMe    bool   `json:"fromMe"`
	ID        string `json:"id"`
}

type DocumentMessage struct {
	Url               string `json:"url"`
	Mimetype          string `json:"mimetype"`
	FileSha256        string `json:"fileSha256"`
	FileLength        string `json:"fileLength"`
	PageCount         int    `json:"pageCount"`
	MediaKey          string `json:"mediaKey"`
	FileName          string `json:"fileName"`
	FileEncSha256     string `json:"fileEncSha256"`
	DirectPath        string `json:"directPath"`
	MediaKeyTimestamp string `json:"mediaKeyTimestamp"`
}

type MessageContent struct {
	Conversation    *string          `json:"conversation,omitempty"`
	DocumentMessage *DocumentMessage `json:"documentMessage,omitempty"`
	Base64          *string          `json:"base64,omitempty"`
}

type SendMessageResponse struct {
	StatusCode   *int          `json:"status_code,omitempty"`
	StatusString *StatusString `json:"status_string,omitempty"`
}

type StatusString struct {
	ContextInfo      *json.RawMessage `json:"contextInfo,omitempty"`
	InstanceID       *string          `json:"instanceId,omitempty"`
	Key              *MessageKey      `json:"key,omitempty"`
	Message          *MessageContent  `json:"message,omitempty"`
	MessageTimestamp *int64           `json:"messageTimestamp,omitempty"`
	MessageType      *string          `json:"messageType,omitempty"`
	PushName         *string          `json:"pushName,omitempty"`
	Source           *string          `json:"source,omitempty"`
	Status           *string          `json:"status,omitempty"`
}
