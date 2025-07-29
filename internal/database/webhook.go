package database

import (
	"database/sql"
	"wasolgo/internal/api"
)

func GetAllWebhooks(db *sql.DB) (*[]api.Webhook, error) {
	rows, err := db.Query("SELECT id, name, url, is_global, conn, send_message, receive_message FROM webhook")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var webhooks []api.Webhook
	for rows.Next() {
		var web api.Webhook
		var conn sql.NullString
		if err := rows.Scan(&web.ID, &web.Name, &web.Url, &web.IsGlobal, &conn, &web.SendMessage, &web.ReceiveMessage); err != nil {
			return nil, err
		}
		if conn.Valid {
			web.Conn = &conn.String
		} else {
			web.Conn = nil
		}
		webhooks = append(webhooks, web)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &webhooks, nil
}
