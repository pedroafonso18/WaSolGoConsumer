package process

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	redis "wasolgo/internal/redis"

	amqp "github.com/rabbitmq/amqp091-go"
	rdb "github.com/redis/go-redis/v9"

	"wasolgo/internal/database"
	"wasolgo/internal/parser"
)

func getStringPointer(m map[string]interface{}, path ...string) (string, bool) {
	var current interface{} = m
	for _, p := range path {
		if m2, ok := current.(map[string]interface{}); ok {
			current = m2[p]
		} else {
			return "", false
		}
	}
	if s, ok := current.(string); ok {
		return s, true
	}
	return "", false
}

func ProcessIncoming(delivery amqp.Delivery, rdb *rdb.Client, db *sql.DB) error {
	message := string(delivery.Body)
	fmt.Printf("Received message: %s", message)

	var value map[string]interface{}
	if err := json.Unmarshal(delivery.Body, &value); err != nil {
		return fmt.Errorf("couldn't unmarshal the json: %w", err)
	}

	chatID, ok := getStringPointer(value, "status_string", "key", "remote_jid")
	if !ok {
		chatID, ok = getStringPointer(value, "data", "key", "remoteJid")
	}
	if !ok {
		chatID, ok = getStringPointer(value, "number")
	}
	if !ok {
		chatID = "unknown_chat"
	}
	chatID = redis.NormalizeChatID(chatID)
	remoteJid := chatID

	isContact := value["name"] != nil && value["number"] != nil && value["created_at"] != nil

	var chatMetadataString string
	var chatMetadata *string
	if isContact {
		contact := make(map[string]interface{})
		for k, v := range value {
			contact[k] = v
		}
		if contact["instance_id"] == nil {
			if instanceID, ok := value["instance_id"]; ok {
				contact["instance_id"] = instanceID
			} else if data, ok := value["data"].(map[string]interface{}); ok {
				if instanceID, ok := data["instanceId"]; ok {
					contact["instance_id"] = instanceID
				}
			}
		}
		b, _ := json.Marshal(contact)
		chatMetadataString = string(b)
		chatMetadata = &chatMetadataString
	}

	var (
		msgID     string
		from      string
		to        string
		text      string
		body      string
		msgType   string
		timestamp string
		extension string
	)
	if data, ok := value["data"].(map[string]interface{}); ok {
		msgID, _ = getStringPointer(data, "key", "id")
		from, _ = getStringPointer(value, "sender")
		to, _ = getStringPointer(data, "key", "remoteJid")
		text, _ = getStringPointer(data, "message", "conversation")
		body = text
		msgType, _ = getStringPointer(data, "messageType")
		if ts, ok := value["date_time"].(string); ok {
			timestamp = ts
		}

		if ext, ok := value["extension"].(string); ok && ext != "" {
			extension = ext
		}

		switch msgType {
		case "imageMessage":
			msgType = "image"
			base64, _ := getStringPointer(data, "message", "base64")
			body = "data:image/png;base64," + base64
			text = "📷 Imagem enviada"
		case "audioMessage":
			msgType = "audio"
			base64, _ := getStringPointer(data, "message", "base64")
			body = "data:audio/ogg;base64," + base64
			text = "Áudio enviado"
		case "documentMessage":
			base64, _ := getStringPointer(data, "message", "base64")
			fileName, _ := getStringPointer(data, "message", "documentMessage", "fileName")
			body = base64
			text = "📄 Documento enviado"
			if extension == "" && fileName != "" {
				if dot := strings.LastIndex(fileName, "."); dot != -1 && dot < len(fileName)-1 {
					extension = fileName[dot+1:]
				}
			}
		}
	}

	normalized := map[string]interface{}{
		"id":        "msg_" + msgID,
		"from":      from,
		"to":        to,
		"text":      text,
		"body":      body,
		"type":      msgType,
		"timestamp": timestamp,
	}
	if extension != "" {
		normalized["extension"] = extension
	}
	messageJSON, _ := json.Marshal(normalized)

	var messageBytes []byte
	if valueBytes, err := json.Marshal(value); err == nil {
		messageBytes = valueBytes
	}

	{
		ctx := context.Background()
		existingChatID, err := redis.FindExistingChatID(ctx, rdb, chatID)
		if err == nil {
			chatKey := "chat:" + existingChatID
			chatJSON, err := rdb.LIndex(ctx, chatKey, 0).Result()
			if err == nil {
				var chatObj map[string]interface{}
				if err := json.Unmarshal([]byte(chatJSON), &chatObj); err == nil {
					situation, _ := chatObj["situation"].(string)
					isActive, _ := chatObj["is_active"].(bool)
					if situation == "finished" || !isActive {
						_ = redis.UpdateChatToOpen(ctx, rdb, chatID)
					}
				}
			}
		}
	}

	err := redis.InsertMessageToChat(
		context.Background(),
		rdb,
		chatID,
		string(messageJSON),
		remoteJid,
		chatMetadata,
		&messageBytes,
	)
	if err != nil {
		return fmt.Errorf("failed to insert message to chat: %w", err)
	}

	msg := parser.Message{
		From:   from,
		To:     to,
		Text:   text,
		ChatID: chatID,
	}
	if db != nil {
		dbErr := database.UpsertMessages(db, &msg)
		if dbErr != nil {
			return fmt.Errorf("failed to insert message into database: %w", dbErr)
		}
	}

	return nil
}
