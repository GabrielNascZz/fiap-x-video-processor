package rabbitmq

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/GabrielNascZz/fiap-x-video-processor/pkg/models"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	ExchangeName      = "video.exchange"
	ProcessQueueName  = "video.process.queue"
	ProcessRoutingKey = "video.process"
	ErrorQueueName    = "video.error.queue"
	ErrorRoutingKey   = "video.error"
)

type RabbitClient struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

func NewRabbitClient(url string) (*RabbitClient, error) {
	var conn *amqp.Connection
	var err error

	for i := 0; i < 10; i++ {
		conn, err = amqp.Dial(url)
		if err == nil {
			break
		}
		log.Printf("Aguardando RabbitMQ ficar disponível (%d/10)...: %v", i+1, err)
		time.Sleep(3 * time.Second)
	}

	if err != nil {
		return nil, fmt.Errorf("falha ao conectar no RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("falha ao abrir canal RabbitMQ: %w", err)
	}

	client := &RabbitClient{
		conn:    conn,
		channel: ch,
	}

	if err := client.setupExchangesAndQueues(); err != nil {
		client.Close()
		return nil, err
	}

	log.Println("✅ Conexão e filas do RabbitMQ configuradas com sucesso!")
	return client, nil
}

func (r *RabbitClient) setupExchangesAndQueues() error {
	err := r.channel.ExchangeDeclare(
		ExchangeName,
		"topic",
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return fmt.Errorf("falha ao declarar exchange: %w", err)
	}

	// Queue for processing videos
	_, err = r.channel.QueueDeclare(
		ProcessQueueName,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return fmt.Errorf("falha ao declarar fila de processamento: %w", err)
	}

	err = r.channel.QueueBind(
		ProcessQueueName,
		ProcessRoutingKey,
		ExchangeName,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("falha ao realizar bind da fila de processamento: %w", err)
	}

	// Queue for notification of errors
	_, err = r.channel.QueueDeclare(
		ErrorQueueName,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return fmt.Errorf("falha ao declarar fila de erro: %w", err)
	}

	err = r.channel.QueueBind(
		ErrorQueueName,
		ErrorRoutingKey,
		ExchangeName,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("falha ao realizar bind da fila de erro: %w", err)
	}

	return nil
}

func (r *RabbitClient) PublishProcessTask(payload models.ProcessVideoTaskPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return r.channel.Publish(
		ExchangeName,
		ProcessRoutingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)
}

func (r *RabbitClient) PublishErrorNotification(payload models.ProcessErrorPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return r.channel.Publish(
		ExchangeName,
		ErrorRoutingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)
}

func (r *RabbitClient) ConsumeProcessQueue() (<-chan amqp.Delivery, error) {
	return r.channel.Consume(
		ProcessQueueName,
		"",    // consumer tag
		false, // auto-ack (manual ack)
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
}

func (r *RabbitClient) ConsumeErrorQueue() (<-chan amqp.Delivery, error) {
	return r.channel.Consume(
		ErrorQueueName,
		"",    // consumer tag
		false, // auto-ack (manual ack)
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
}

func (r *RabbitClient) Close() {
	if r.channel != nil {
		r.channel.Close()
	}
	if r.conn != nil {
		r.conn.Close()
	}
}
