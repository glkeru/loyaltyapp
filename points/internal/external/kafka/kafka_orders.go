package points

import (
	"context"
	"fmt"
	"os"

	"github.com/segmentio/kafka-go"
)

type KafkaOrder struct {
	reader *kafka.Reader
}

func GetNewReader(topic string) (reader *KafkaOrder, err error) {
	// config
	kafkaurl := os.Getenv("KAFKA_ORDER_URL")
	if kafkaurl == "" {
		return nil, fmt.Errorf("env KAFKA_ORDER_URL is not set")
	}
	kafkaport := os.Getenv("KAFKA_ORDER_PORT")
	if kafkaurl == "" {
		return nil, fmt.Errorf("env KAFKA_ORDER_PORT is not set")
	}

	kafkaconfig := kafka.ReaderConfig{
		Brokers: []string{kafkaurl + ":" + kafkaport},
		Topic:   topic,
		GroupID: "orders_loyalty",
	}
	return &KafkaOrder{kafka.NewReader(kafkaconfig)}, nil
}

func (k *KafkaOrder) GetNewMessage(ctx context.Context) (orderJson string, err error) {
	msg, err := k.reader.ReadMessage(ctx)
	if err != nil {
		return "", err
	}
	return string(msg.Value), nil
}

func (k *KafkaOrder) CloseReader() {
	k.reader.Close()
}
