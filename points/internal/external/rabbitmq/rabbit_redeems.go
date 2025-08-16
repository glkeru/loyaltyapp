package points

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitConsumer struct {
	conn  *amqp.Connection
	ch    *amqp.Channel
	Msg   <-chan amqp.Delivery
	chout *amqp.Channel
}

const queue = "redeems"
const queueout = "confirms"

func NewRabbitConsumer() (rabbit *RabbitConsumer, err error) {
	// config
	rabbiturl := os.Getenv("LOYALTY_RABBIT_URL")
	if rabbiturl == "" {
		return nil, fmt.Errorf("env LOYALTY_RABBIT_URL is not set")
	}
	conn, err := amqp.Dial(rabbiturl)
	if err != nil {
		return nil, err
	}
	// канал для входящих
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}
	_, err = ch.QueueDeclare(
		queue, // name
		false, // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}

	// канал для исходящих
	chout, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}
	_, err = chout.QueueDeclare(
		queueout, // name
		false,    // durable
		false,    // delete when unused
		false,    // exclusive
		false,    // no-wait
		nil,      // arguments
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}

	msg, err := ch.Consume(
		queue, // queue
		"",    // consumer
		true,  // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}

	return &RabbitConsumer{conn, ch, msg, chout}, nil
}
func (r *RabbitConsumer) Close() {
	r.ch.Close()
	r.conn.Close()
}

type RedeemConfirm struct {
	redeemId string
	success  bool
}

// подтверждение списания
func (r *RabbitConsumer) Processed(ctx context.Context, redeemId string, success bool) error {
	st := &RedeemConfirm{redeemId, success}
	msg, err := json.Marshal(st)
	if err != nil {
		return err
	}

	err = r.chout.PublishWithContext(ctx,
		"",       // exchange
		queueout, // routing key
		false,    // mandatory
		false,    // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        []byte(msg),
		})
	if err != nil {
		return err
	}
	return nil
}
