package rabbit

import (
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func connectRabbitMQ(rabbitURL, queueName string) (*amqp.Connection, error) {
	conn, err := amqp.DialConfig(rabbitURL, amqp.Config{
		Properties: amqp.Table{
			"connection_name": queueName,
		},
	})
	return conn, err
}

func setupConsumer(conn *amqp.Connection, queueName string) (<-chan amqp.Delivery, *amqp.Channel, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, nil, err
	}

	if err := ch.Qos(1, 0, false); err != nil {
		ch.Close()
		return nil, nil, err
	}

	args := amqp.Table{
		"x-queue-type": "quorum",
	}
	_, err = ch.QueueDeclare(
		queueName,
		true,
		false,
		false,
		false,
		args,
	)
	if err != nil {
		ch.Close()
		return nil, nil, err
	}

	msgs, err := ch.Consume(
		queueName,
		"WasolConsumer",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		ch.Close()
		return nil, nil, err
	}

	return msgs, ch, nil
}

func CreateRabbitMQConsumer(rabbitURL, queueName string) (<-chan amqp.Delivery, *amqp.Connection, *amqp.Channel) {
	for {
		conn, err := connectRabbitMQ(rabbitURL, queueName)
		if err != nil {
			log.Printf("Failed to connect to RabbitMQ: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}
		log.Println("RabbitMQ connection established")

		msgs, ch, err := setupConsumer(conn, queueName)
		if err != nil {
			log.Printf("Failed to set up consumer: %v", err)
			conn.Close()
			time.Sleep(5 * time.Second)
			continue
		}
		log.Println("Consumer set up successfully")
		return msgs, conn, ch
	}
}
