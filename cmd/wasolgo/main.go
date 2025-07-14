package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
	"wasolgo/internal/config"
	consumer "wasolgo/internal/consume"
	"wasolgo/internal/database"
	"wasolgo/internal/redis"
)

func main() {
	env, err := config.LoadEnv()
	if err != nil {
		fmt.Printf("Error: Couldn't retrieve .env: %v", err)
	}

	log.Print("Starting application - Check Logs below...")
	log.Print("Starting WaSolConsumer")

	redisConn, err := redis.ConnectRedis(env.RedisUrl)
	if err != nil {
		log.Fatalf("ERROR: Couldn't connect to Redis: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	for {
		dbClient, err := database.ConnectDb(env.DbUrl)
		if err != nil {
			log.Printf("ERROR: Couldn't connect to Database, retrying... : %v", err)
			time.Sleep(30 * time.Second)
			continue
		}

		log.Print("Setting up Outgoing and Incoming Request consumers...")

		var wg sync.WaitGroup
		errCh := make(chan error, 4)

		queues := []string{"outgoing_requests", "incoming_requests", "evolution.messages.upsert", "evolution.send.message"}

		for _, queueName := range queues {
			wg.Add(1)
			go func(queue string) {
				defer wg.Done()
				if err := consumer.RunConsumer(ctx, env.RabbitUrl, dbClient, queue, redisConn); err != nil {
					errCh <- fmt.Errorf("consumer %s failed: %w", queue, err)
				}
			}(queueName)
		}

		go func() {
			wg.Wait()
			close(errCh)
		}()

		select {
		case <-ctx.Done():
			log.Print("Shutdown requested, exiting main loop.")
			return
		case err := <-errCh:
			log.Printf("Error in consumer loop: %v", err)
			fmt.Printf("ERROR: Consumer loop failed: %v\n", err)
			log.Print("Reconnecting in 5 seconds...")
			time.Sleep(5 * time.Second)
		}
	}
}
