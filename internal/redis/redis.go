package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/redis/go-redis/v9"
)

func ConnectRedis(redisURL string) (*redis.Client, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	client := redis.NewClient(opt)
	return client, nil
}

func NormalizeChatID(jid string) string {
	parts := strings.SplitN(jid, "@", 2)
	if len(parts) == 2 {
		number, domain := parts[0], parts[1]
		if strings.HasPrefix(number, "55") && len(number) >= 12 {
			countryCode := number[:2]
			areaCode := number[2:4]
			rest := number[4:]
			if !strings.HasPrefix(rest, "9") {
				rest = "9" + rest
			}
			normalizedNumber := countryCode + areaCode + rest
			return fmt.Sprintf("%s@%s", normalizedNumber, domain)
		}
	}
	return jid
}

func EnsureChatExists(ctx context.Context, rdb *redis.Client, chatID, remoteJid string, chatMetadata *string, messageData *[]byte) error {
	normChatID := NormalizeChatID(chatID)
	chatKey := fmt.Sprintf("chat:%s", normChatID)
	exists, err := rdb.Exists(ctx, chatKey).Result()
	if err != nil {
		return err
	}

	if exists == 0 {
		var chatData string
		if chatMetadata != nil {
			chatData = *chatMetadata
		} else {
			number := strings.SplitN(remoteJid, "@", 2)[0]
			instanceID := ""
			if messageData != nil {
				var value map[string]interface{}
				if err := json.Unmarshal(*messageData, &value); err == nil {
					if apikey, ok := value["apikey"].(string); ok {
						instanceID = apikey
					}
				}
			}
			meta := map[string]interface{}{
				"id":          normChatID,
				"situation":   "enqueued",
				"is_active":   true,
				"agent_id":    nil,
				"tabulation":  nil,
				"instance_id": instanceID,
				"number":      number,
			}
			b, _ := json.Marshal(meta)
			chatData = string(b)
		}
		if _, err := rdb.RPush(ctx, chatKey, chatData).Result(); err != nil {
			return err
		}
		log.Printf("Created new chat entry in Redis (as list): %s", chatKey)
		if _, err := rdb.SAdd(ctx, "chats", normChatID).Result(); err != nil {
			return err
		}
		log.Printf("Added chat_id %s to 'chats' set", normChatID)
	} else {
		log.Printf("Chat entry already exists in Redis: %s", chatKey)
	}
	return nil
}

func InsertMessageToChat(
	ctx context.Context,
	rdb *redis.Client,
	chatID string,
	messageJSON string,
	remoteJid string,
	chatMetadata *string,
	messageData *[]byte,
) error {
	normChatID := NormalizeChatID(chatID)
	log.Printf("Inserting message into chat:%s for remote_jid:%s", normChatID, remoteJid)
	if err := EnsureChatExists(ctx, rdb, normChatID, remoteJid, chatMetadata, messageData); err != nil {
		log.Printf("Failed to ensure chat exists: %v", err)
		return err
	}
	key := fmt.Sprintf("chat:%s:messages", normChatID)
	log.Printf("Pushing message to Redis list: %s", key)
	if _, err := rdb.RPush(ctx, key, messageJSON).Result(); err != nil {
		return err
	}
	log.Printf("Successfully inserted message into Redis for chat:%s", normChatID)
	return nil
}
