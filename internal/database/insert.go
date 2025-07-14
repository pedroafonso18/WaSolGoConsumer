package database

import (
	"database/sql"
	"fmt"
	"wasolgo/internal/parser"
)

func UpsertChat(db *sql.DB, chat *parser.Chat) error {
	if chat.Tabulation == nil {
		query := "INSERT INTO chats (id, situation, is_active, agent_id, customer_id, instance_id) VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (id) DO UPDATE SET situation = $2, is_active = $3, agent_id = $4, customer_id = $5, instance_id = $6"
		_, err := db.Exec(query, chat.ID, chat.Situation, chat.IsActive, chat.AgentID, chat.CustomerID, chat.InstanceID)
		if err != nil {
			return fmt.Errorf("couldn't insert chat into database: %w", err)
		}
	} else {
		query := `
INSERT INTO chats (id, situation, is_active, agent_id, tabulation, customer_id, instance_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (id) DO UPDATE
SET situation = $2, is_active = $3, agent_id = $4, tabulation = $5, customer_id = $6, instance_id = $7
`
		_, err := db.Exec(query, chat.ID, chat.Situation, chat.IsActive, chat.AgentID, chat.Tabulation, chat.CustomerID, chat.InstanceID)
		if err != nil {
			return fmt.Errorf("couldn't insert chat into database: %w", err)
		}
	}
	return nil
}

func UpsertMessages(db *sql.DB, msg *parser.Message) error {
	query := "INSERT INTO messages (id, \"from\", \"to\", text, delivered, chat_id) VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (id) DO UPDATE SET \"from\" = $2, \"to\" = $3, text = $4, delivered = $5, chat_id = $6"
	_, err := db.Exec(query, msg.ID, msg.From, msg.To, msg.Text, msg.Delivered, msg.ChatID)
	if err != nil {
		return fmt.Errorf("couldn't insert message into database: %w", err)
	}
	return nil
}

func UpsertCustomer(db *sql.DB, customer *parser.Customer) error {
	if customer.LastChatID == nil {
		query := "INSERT INTO customers (id, name, number) VALUES ($1, $2, $3) ON CONFLICT (id) DO UPDATE SET name = $2, number = $3"
		_, err := db.Exec(query, customer.ID, customer.Name, customer.Number)
		if err != nil {
			return fmt.Errorf("couldn't insert customer into database: %w", err)
		}
		return nil
	} else {
		query := "INSERT INTO customers (id, name, number, last_chat_id) VALUES ($1, $2, $3, $4) ON CONFLICT (id) DO UPDATE SET name = $2, number = $3, last_chat_Id = $4"
		_, err := db.Exec(query, customer.ID, customer.Name, customer.Number, customer.LastChatID)
		if err != nil {
			return fmt.Errorf("couldn't insert customer into database: %w", err)
		}
		return nil
	}
}
