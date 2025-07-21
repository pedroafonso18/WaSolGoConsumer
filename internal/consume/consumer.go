package consumer

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"

	"wasolgo/internal/parser"
	"wasolgo/internal/process"
	"wasolgo/internal/redis"

	amqp "github.com/rabbitmq/amqp091-go"
	rdb "github.com/redis/go-redis/v9"
)

func RunConsumer(
	ctx context.Context,
	rabbitURL string,
	dbClient *sql.DB,
	queueName string,
	redisConn *rdb.Client,
) error {
	conn, err := amqp.Dial(rabbitURL)
	if err != nil {
		return err
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	args := amqp.Table{
		"x-queue-type": "quorum",
	}
	_, err = ch.QueueDeclare(
		queueName,
		true,  // durable
		false, // autoDelete
		false, // exclusive
		false, // noWait
		args,  // arguments
	)
	if err != nil {
		return err
	}

	msgs, err := ch.Consume(
		queueName,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Consumer ready, waiting for webhooks... Press Ctrl+C to exit")

	for {
		select {
		case <-ctx.Done():
			log.Println("Context cancelled, shutting down consumer")
			return nil
		case sig := <-sigCh:
			log.Printf("Received signal: %v, shutting down", sig)
			return nil
		case delivery, ok := <-msgs:
			if !ok {
				log.Println("Consumer channel closed")
				return nil
			}

			go func(delivery amqp.Delivery) {
				log.Printf("[DEBUG] Received delivery for queue: %s", queueName)
				var err error
				switch queueName {
				case "incoming_requests", "evolution.messages.upsert":
					err = process.ProcessIncoming(delivery, redisConn, dbClient)
				case "outgoing_requests":
					err = process.ProcessOutgoing(delivery, dbClient)
				case "evolution.send.message":
					type Key struct {
						RemoteJid string `json:"remote_jid"`
					}
					type StatusString struct {
						Key     *Key        `json:"key"`
						Message interface{} `json:"message"`
					}
					type SendMessageResponse struct {
						StatusString *StatusString `json:"status_string"`
					}

					var resp SendMessageResponse
					if err := json.Unmarshal(delivery.Body, &resp); err != nil {
						log.Printf("Failed to deserialize SendMessageResponse: %v", err)
						delivery.Nack(false, false)
						return
					}
					if resp.StatusString != nil && resp.StatusString.Key != nil && resp.StatusString.Message != nil {
						log.Printf("[DEBUG] Entered evolution.send.message handler, resp: %+v", resp)
						chatID := redis.NormalizeChatID(resp.StatusString.Key.RemoteJid)
						remoteJid := chatID

						var msgContent parser.MessageContent
						msgBytes, _ := json.Marshal(resp.StatusString.Message)
						_ = json.Unmarshal(msgBytes, &msgContent)

						var messageMap map[string]interface{}
						_ = json.Unmarshal(msgBytes, &messageMap)

						log.Printf("[DEBUG] resp.StatusString.Message: %v", resp.StatusString.Message)
						log.Printf("[DEBUG] msgContent: %+v", msgContent)
						log.Printf("[DEBUG] messageMap: %+v", messageMap)

						var base64Body string
						if msgContent.DocumentMessage != nil && msgContent.Base64 != nil {
							base64Body = *msgContent.Base64
						} else if b64, ok := messageMap["base64"]; ok {
							if b64Str, ok := b64.(string); ok {
								base64Body = b64Str
							}
						}
						if base64Body != "" {
							messageMap["body"] = base64Body
						}

						messageJSON, err := json.Marshal(messageMap)
						if err != nil {
							log.Printf("Failed to marshal message: %v", err)
							delivery.Nack(false, false)
							return
						}

						log.Printf("[DEBUG] Final messageJSON to Redis: %s", string(messageJSON))

						var chatKeyToUse string
						possibleIDs := redis.PossibleChatIDs(resp.StatusString.Key.RemoteJid)
						for _, id := range possibleIDs {
							key := "chat:" + id
							exists, err := redisConn.Exists(context.Background(), key).Result()
							if err == nil && exists > 0 {
								chatKeyToUse = id
								break
							}
						}
						if chatKeyToUse == "" {
							chatKeyToUse = chatID
						}

						if err := redis.InsertMessageToChat(
							context.Background(),
							redisConn,
							chatKeyToUse,
							string(messageJSON),
							remoteJid,
							nil,
							nil,
						); err != nil {
							log.Printf("Failed to insert message to Redis: %v", err)
							delivery.Nack(false, false)
							return
						}
					}
					if err := delivery.Ack(false); err != nil {
						log.Printf("Failed to acknowledge message: %v", err)
					}
					return
				default:
					log.Printf("[DEBUG] Unhandled queueName: %s", queueName)
				}
				if err != nil {
					log.Printf("Error processing message: %v", err)
					delivery.Nack(false, false)
				} else {
					if err := delivery.Ack(false); err != nil {
						log.Printf("Failed to acknowledge message: %v", err)
					}
				}
			}(delivery)
		}
	}
}
