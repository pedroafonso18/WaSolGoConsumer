package process

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"wasolgo/internal/api"
	"wasolgo/internal/database"
	"wasolgo/internal/parser"

	amqp "github.com/rabbitmq/amqp091-go"
)

func ProcessOutgoing(delivery amqp.Delivery, client *sql.DB) error {
	message := string(delivery.Body)
	fmt.Printf("Received message: %s", message)

	if strings.Contains(message, "upsertChat") {
		fmt.Print("Starting UpsertChat process...")
		var chat parser.Chat
		if err := json.Unmarshal(delivery.Body, &chat); err != nil {
			return fmt.Errorf("failed to unmarshal upsertChat message: %w", err)
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
		if err := json.Unmarshal(delivery.Body, &customer); err != nil {
			return fmt.Errorf("failed to unmarshal upsertCustomer message: %w", err)
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
		if err := json.Unmarshal(delivery.Body, &msg); err != nil {
			return fmt.Errorf("failed to unmarshal SendMessage message: %w", err)
		}
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
		if err := json.Unmarshal(delivery.Body, &req); err != nil {
			return fmt.Errorf("failed to unmarshal SendRequest message: %w", err)
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
