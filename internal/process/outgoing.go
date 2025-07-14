package process

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"wasolgo/internal/api"
	"wasolgo/internal/database"
	"wasolgo/internal/parser"

	amqp "github.com/rabbitmq/amqp091-go"
)

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case bool:
			return val
		case string:
			b, _ := strconv.ParseBool(val)
			return b
		}
	}
	return false
}

func getInt(m map[string]interface{}, key string) int {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case int:
			return val
		case float64:
			return int(val)
		case string:
			if i, err := strconv.Atoi(val); err == nil {
				return i
			}
		}
	}
	return 0
}

func ProcessOutgoing(delivery amqp.Delivery, client *sql.DB) error {
	message := string(delivery.Body)
	fmt.Printf("Received message: %s", message)

	var envelope map[string]interface{}
	if err := json.Unmarshal(delivery.Body, &envelope); err != nil {
		return fmt.Errorf("failed to unmarshal envelope: %w", err)
	}

	bodyRaw, ok := envelope["body"]
	if !ok {
		return fmt.Errorf("missing 'body' field in message: %s", message)
	}

	bodyBytes, err := json.Marshal(bodyRaw)
	if err != nil {
		return fmt.Errorf("failed to marshal body field: %w", err)
	}

	if strings.Contains(message, "upsertChat") {
		fmt.Print("Starting UpsertChat process...")
		var chatMap map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &chatMap); err != nil {
			return fmt.Errorf("failed to unmarshal upsertChat body: %w", err)
		}

		var chat parser.Chat
		chat.ID = getString(chatMap, "id")
		chat.Situation = getString(chatMap, "situation")
		chat.InstanceID = getString(chatMap, "instance_id")
		chat.AgentID = getString(chatMap, "agent_id")
		chat.CustomerID = getString(chatMap, "customer_id")

		chat.IsActive = getBool(chatMap, "is_active")

		if tab, ok := chatMap["tabulation"].(string); ok && tab != "" {
			chat.Tabulation = &tab
		}

		err := database.UpsertChat(client, &chat)
		if err != nil {
			return fmt.Errorf("error on upserting chat into the db: %w", err)
		} else {
			fmt.Print("Successfully inserted chat into db!")
			return nil
		}
	} else if strings.Contains(message, "upsertCustomer") {
		fmt.Print("Starting UpsertCustomer process...")
		var customer parser.Customer
		if err := json.Unmarshal(bodyBytes, &customer); err != nil {
			return fmt.Errorf("failed to unmarshal upsertCustomer body: %w", err)
		}

		if customer.LastChatID != nil && *customer.LastChatID == "" {
			customer.LastChatID = nil
		}
		err := database.UpsertCustomer(client, &customer)
		if err != nil {
			return fmt.Errorf("error on upserting customer into the db: %w", err)
		} else {
			fmt.Print("Successfully inserted customer into db!")
			return nil
		}
	} else if strings.Contains(message, "sendMessage") {
		fmt.Print("Starting SendMessage process...")
		var msg parser.Message
		if err := json.Unmarshal(bodyBytes, &msg); err != nil {
			return fmt.Errorf("failed to unmarshal SendMessage body: %w", err)
		}
		err := database.UpsertMessages(client, &msg)
		if err != nil {
			return fmt.Errorf("error on upserting message into the db: %w", err)
		} else {
			fmt.Print("Successfully inserted message into db!")
			return nil
		}
	} else if strings.Contains(message, "upsertMessage") {
		fmt.Print("Starting UpsertMessage process...")
		var msgMap map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &msgMap); err != nil {
			return fmt.Errorf("failed to unmarshal upsertMessage body: %w", err)
		}

		var msg parser.Message
		msg.ID = getInt(msgMap, "id")
		msg.From = getString(msgMap, "from")
		msg.To = getString(msgMap, "to")
		msg.Text = getString(msgMap, "text")
		msg.ChatID = getString(msgMap, "chat_id")
		msg.Delivered = getBool(msgMap, "delivered")

		err := database.UpsertMessages(client, &msg)
		if err != nil {
			return fmt.Errorf("error on upserting message into the db: %w", err)
		} else {
			fmt.Print("Successfully inserted message into db!")
			return nil
		}
	} else if strings.Contains(message, "sendRequest") {
		fmt.Print("Starting SendRequest process...")
		var req parser.Request
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			return fmt.Errorf("failed to unmarshal SendRequest body: %w", err)
		}
		err := api.SendRequest(&req)
		if err != nil {
			return fmt.Errorf("error on sending request: %w", err)
		} else {
			fmt.Print("Successfully sent request!")
			return nil
		}
	}
	return fmt.Errorf("unknown message type. Message content: %s", message)
}
