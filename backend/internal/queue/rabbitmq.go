package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/fredyxander/okf-platform/backend/internal/domain"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	JobsQueue      = "document_jobs"
	JobsRetryQueue = "document_jobs_retry"
	JobsDLQ        = "document_jobs_dlq"
)

// RabbitMQ representa nuestra conexión con el broker.
type RabbitMQ struct {
	connection *amqp.Connection
	channel    *amqp.Channel
}

// NewRabbitMQ crea una nueva conexión con RabbitMQ.
func NewRabbitMQ(url string) (*RabbitMQ, error) {
	connection, err := amqp.Dial(url)

	if err != nil {
		return nil, fmt.Errorf("connect to RabbitMQ: %w", err)
	}

	channel, err := connection.Channel()

	if err != nil {
		connection.Close()

		return nil, fmt.Errorf("open RabbitMQ channel: %w", err)
	}

	// Cola principal de procesamiento.
	_, err = channel.QueueDeclare(
		JobsQueue,
		true,
		false,
		false,
		false,
		nil,
	)

	if err != nil {
		channel.Close()
		connection.Close()

		return nil, fmt.Errorf("declare queue: %w", err)
	}

	// Cola de retry diferido.
	// Los mensajes permanecen aquí 30 segundos y, al expirar,
	// RabbitMQ los devuelve automáticamente a JobsQueue.
	_, err = channel.QueueDeclare(
		JobsRetryQueue,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		amqp.Table{
			"x-message-ttl":             int32(30000),
			"x-dead-letter-exchange":    "",
			"x-dead-letter-routing-key": JobsQueue,
		},
	)

	if err != nil {
		channel.Close()
		connection.Close()

		return nil, fmt.Errorf("declare retry queue: %w", err)
	}

	_, err = channel.QueueDeclare(
		JobsDLQ,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	)

	if err != nil {
		channel.Close()
		connection.Close()

		return nil, fmt.Errorf("declare DLQ: %w", err)
	}

	return &RabbitMQ{
		connection: connection,
		channel:    channel,
	}, nil
}

// Close libera correctamente los recursos.
func (r *RabbitMQ) Close() {
	if r.channel != nil {
		r.channel.Close()
	}

	if r.connection != nil {
		r.connection.Close()
	}
}

func (r *RabbitMQ) PublishJob(
	ctx context.Context,
	job domain.JobMessage,
) error {

	body, err := json.Marshal(job)

	if err != nil {
		return fmt.Errorf("encode job message: %w", err)
	}

	err = r.channel.PublishWithContext(
		ctx,
		"",
		JobsQueue,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)

	if err != nil {
		return fmt.Errorf("publish job: %w", err)
	}

	return nil
}

// Worker escucha
func (r *RabbitMQ) ConsumeJobs() (<-chan amqp.Delivery, error) {
	err := r.channel.Qos(
		1,     // prefetchCount - RabbitMQ permitirá a este consumidor tener como máximo un mensaje entregado pero todavía sin ACK.
		//Job B permanece en RabbitMQ -> ACk Job A -> RabbitMQ entrega siguiente mensaje al worker Job B, una vez completo el A
		0,     // prefetchSize
		false, // global
	)
	if err != nil {
		return nil, fmt.Errorf("configure consumer qos: %w", err)
	}

	messages, err := r.channel.Consume(
		JobsQueue,
		"",
		false,
		false,
		false,
		false,
		nil,
	)

	if err != nil {
		return nil, fmt.Errorf("consume jobs: %w", err)
	}

	return messages, nil
}

//retry de conexion
func NewRabbitMQWithRetry(
	url string,
	maxRetries int,
	delay time.Duration,
) (*RabbitMQ, error) {

	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {

		rabbitMQ, err := NewRabbitMQ(url)

		if err == nil {
			log.Printf(
				"connected to RabbitMQ on attempt %d",
				attempt,
			)

			return rabbitMQ, nil
		}

		lastErr = err

		log.Printf(
			"RabbitMQ connection attempt %d/%d failed: %v",
			attempt,
			maxRetries,
			err,
		)

		if attempt < maxRetries {
			time.Sleep(delay)
		}
	}

	return nil, fmt.Errorf(
		"could not connect to RabbitMQ after %d attempts: %w",
		maxRetries,
		lastErr,
	)
}

func (r *RabbitMQ) PublishJobRetry(
	ctx context.Context,
	job domain.JobMessage,
) error {
	job.Attempt++

	body, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("encode retry job message: %w", err)
	}

	err = r.channel.PublishWithContext(
		ctx,
		"",
		JobsRetryQueue,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)

	if err != nil {
		return fmt.Errorf("publish retry job: %w", err)
	}

	return nil
}

func (r *RabbitMQ) PublishJobDLQ(
	ctx context.Context,
	job domain.JobMessage,
) error {
	body, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("encode DLQ job message: %w", err)
	}

	err = r.channel.PublishWithContext(
		ctx,
		"",
		JobsDLQ,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)

	if err != nil {
		return fmt.Errorf("publish job to DLQ: %w", err)
	}

	return nil
}

func (r *RabbitMQ) PublishJobDeferred(
	ctx context.Context,
	job domain.JobMessage,
) error {
	body, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("encode deferred job message: %w", err)
	}

	err = r.channel.PublishWithContext(
		ctx,
		"",
		JobsRetryQueue,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)

	if err != nil {
		return fmt.Errorf("publish deferred job: %w", err)
	}

	return nil
}