package process

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	redis "wasolgo/internal/redis"

	"wasolgo/internal/api"

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

		webhooks, err := database.GetAllWebhooks(db)
		if err != nil {
			fmt.Printf("[DEBUG] Failed to get webhooks: %v", err)
		} else if webhooks == nil || len(*webhooks) == 0 {
			fmt.Printf("[DEBUG] No webhooks configured")
		} else {
			fmt.Printf("[DEBUG] Found %d webhooks", len(*webhooks))
			ctx := context.Background()
			var department, agent, tag string
			var isOpen bool
			if chatInfo, err := redis.GetChat(ctx, rdb, chatID); err == nil {
				if dep, ok := chatInfo["department"].(string); ok {
					department = dep
				}
				if ag, ok := chatInfo["agent_id"].(string); ok {
					agent = ag
				}
				if tg, ok := chatInfo["tags"].(string); ok {
					tag = tg
				}
				if open, ok := chatInfo["is_active"].(bool); ok {
					isOpen = open
				}
			}
			connID, _ := getStringPointer(value, "instance_id")
			if connID == "" {
				connID, _ = getStringPointer(value, "data", "instanceId")
			}
			payload := api.WebhookMessage{
				Conn:       connID,
				Message:    text,
				SentBy:     from,
				Department: department,
				Agent:      agent,
				Tag:        tag,
				IsOpen:     isOpen,
			}
			webhookSent := false
			for _, wh := range *webhooks {
				if wh.ReceiveMessage {
					if wh.Conn != nil && connID != "" && *wh.Conn != connID && !wh.IsGlobal {
						fmt.Printf("[DEBUG] Webhook filtered out: conn mismatch (webhook: %s, message: %s)", *wh.Conn, connID)
						continue
					}
					fmt.Printf("[DEBUG] Sending webhook to: %s", wh.Url)
					go api.SendWebhook(wh.Url, &payload)
					webhookSent = true
				}
			}
			if !webhookSent {
				fmt.Printf("[DEBUG] No webhooks were sent (all filtered out or not configured for message)")
			}
		}
	} else {
		fmt.Printf("[DEBUG] Database is nil, skipping webhook logic")
	}

	return nil
}
