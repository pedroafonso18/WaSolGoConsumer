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
		if strings.HasPrefix(number, "55") && len(number) > 4 {
			countryCode := number[:2]
			areaCode := number[2:4]
			numPart := number[4:]
			// If numPart is 8 digits, add a 9
			if len(numPart) == 8 {
				numPart = "9" + numPart
			}
			// If numPart is 9 digits and starts with 9, use as is
			normalizedNumber := countryCode + areaCode + numPart
			return fmt.Sprintf("%s@%s", normalizedNumber, domain)
		}
	}
	return jid
}

// Helper to generate both possible chat IDs (original and normalized)
func PossibleChatIDs(jid string) []string {
	parts := strings.SplitN(jid, "@", 2)
	if len(parts) != 2 {
		return []string{jid}
	}
	number, domain := parts[0], parts[1]
	if strings.HasPrefix(number, "55") && len(number) > 4 {
		countryCode := number[:2]
		areaCode := number[2:4]
		numPart := number[4:]
		ids := []string{}
		// Original
		ids = append(ids, fmt.Sprintf("%s@%s", number, domain))
		// Normalized (if not already normalized)
		if len(numPart) == 8 {
			normalized := countryCode + areaCode + "9" + numPart
			ids = append(ids, fmt.Sprintf("%s@%s", normalized, domain))
		} else if len(numPart) == 9 && strings.HasPrefix(numPart, "9") {
			// Already normalized, just add as possible (if not already in list)
			normalized := countryCode + areaCode + numPart
			if normalized != number {
				ids = append(ids, fmt.Sprintf("%s@%s", normalized, domain))
			}
		}
		return ids
	}
	return []string{jid}
}

// FindExistingChatID checks Redis for both possible chat IDs (with and without the 9) and returns the one that exists, or the normalized one if neither exists.
func FindExistingChatID(ctx context.Context, rdb *redis.Client, chatID string) (string, error) {
	possibleIDs := PossibleChatIDs(chatID)
	log.Printf("[FindExistingChatID] Checking possible chat IDs for %s: %v", chatID, possibleIDs)
	for _, id := range possibleIDs {
		key := "chat:" + id
		exists, err := rdb.Exists(ctx, key).Result()
		if err != nil {
			log.Printf("[FindExistingChatID] Error checking key %s: %v", key, err)
			return "", err
		}
		log.Printf("[FindExistingChatID] Key %s exists: %d", key, exists)
		if exists > 0 {
			log.Printf("[FindExistingChatID] Found existing chat key: %s", key)
			return id, nil
		}
	}
	// If none exist, return the normalized one
	normalized := NormalizeChatID(chatID)
	log.Printf("[FindExistingChatID] No existing chat found, using normalized: %s", normalized)
	return normalized, nil
}

func EnsureChatExists(ctx context.Context, rdb *redis.Client, chatID, remoteJid string, chatMetadata *string, messageData *[]byte) error {
	existingChatID, err := FindExistingChatID(ctx, rdb, chatID)
	if err != nil {
		return err
	}
	chatKey := "chat:" + existingChatID
	exists, err := rdb.Exists(ctx, chatKey).Result()
	if err != nil {
		return err
	}
	foundInSet, err := rdb.SIsMember(ctx, "chats", existingChatID).Result()
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
				"id":          existingChatID,
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
		if !foundInSet {
			if _, err := rdb.SAdd(ctx, "chats", existingChatID).Result(); err != nil {
				return err
			}
			log.Printf("Added chat_id %s to 'chats' set", existingChatID)
		}
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
	existingChatID, err := FindExistingChatID(ctx, rdb, chatID)
	if err != nil {
		log.Printf("Failed to resolve existing chat ID: %v", err)
		return err
	}
	log.Printf("Inserting message into chat:%s for remote_jid:%s", existingChatID, remoteJid)
	if err := EnsureChatExists(ctx, rdb, existingChatID, remoteJid, chatMetadata, messageData); err != nil {
		log.Printf("Failed to ensure chat exists: %v", err)
		return err
	}
	key := fmt.Sprintf("chat:%s:messages", existingChatID)
	log.Printf("Pushing message to Redis list: %s", key)
	if _, err := rdb.RPush(ctx, key, messageJSON).Result(); err != nil {
		return err
	}
	log.Printf("Successfully inserted message into Redis for chat:%s", existingChatID)
	return nil
}
