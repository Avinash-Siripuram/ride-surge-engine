package kafka

import (
	"context"
	"fmt"
	"log"

	"github.com/segmentio/kafka-go"
)

var Writer *kafka.Writer
var ctx = context.Background()

const (
	TopicLocationUpdated = "driver-location-updated"
	TopicRideRequested   = "ride-requested"
	TopicRideMatched     = "ride-matched"
	TopicSurgeCalculated = "surge-calculated"
)

// InitKafka initializes the Kafka writer connection pool
func InitKafka(broker string) {
	// Create Kafka topics if they don't exist
	// In local dev, topic auto-creation is enabled in Kafka, but we can configure writer directly
	Writer = &kafka.Writer{
		Addr:     kafka.TCP(broker),
		Balancer: &kafka.LeastBytes{},
	}
	log.Printf("Kafka client initialized for broker %s", broker)
}

// PublishEvent sends a message to a specific topic
func PublishEvent(topic string, key string, val []byte) error {
	if Writer == nil {
		return fmt.Errorf("kafka writer not initialized")
	}

	err := Writer.WriteMessages(ctx, kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: val,
	})

	if err != nil {
		log.Printf("Failed to publish event to topic %s: %v", topic, err)
		return err
	}

	return nil
}

// CloseKafka cleans up the Kafka writer connections
func CloseKafka() {
	if Writer != nil {
		if err := Writer.Close(); err != nil {
			log.Printf("Error closing Kafka writer: %v", err)
		}
	}
}
