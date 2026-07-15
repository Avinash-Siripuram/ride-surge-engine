package kafka

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"

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

// InitKafka initializes the Kafka writer connection pool and pre-creates topics
func InitKafka(broker string) {
	// Pre-create topics programmatically
	topics := []string{TopicLocationUpdated, TopicRideRequested, TopicRideMatched, TopicSurgeCalculated}
	for _, t := range topics {
		ensureTopicExists(broker, t)
	}

	Writer = &kafka.Writer{
		Addr:                   kafka.TCP(broker),
		Balancer:               &kafka.LeastBytes{},
		AllowAutoTopicCreation: true,
	}
	log.Printf("Kafka client initialized for broker %s", broker)
}

func ensureTopicExists(broker string, topic string) {
	conn, err := kafka.Dial("tcp", broker)
	if err != nil {
		log.Printf("Failed to dial Kafka broker: %v", err)
		return
	}
	defer conn.Close()

	controller, err := conn.Controller()
	if err != nil {
		log.Printf("Failed to get Kafka controller: %v", err)
		return
	}

	controllerAddr := net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port))
	controllerConn, err := kafka.Dial("tcp", controllerAddr)
	if err != nil {
		log.Printf("Failed to dial controller at %s: %v", controllerAddr, err)
		return
	}
	defer controllerConn.Close()

	topicConfigs := []kafka.TopicConfig{
		{
			Topic:             topic,
			NumPartitions:     1,
			ReplicationFactor: 1,
		},
	}

	err = controllerConn.CreateTopics(topicConfigs...)
	if err != nil {
		log.Printf("Topic %s verification: %v (might already exist)", topic, err)
	} else {
		log.Printf("Topic %s successfully verified/created", topic)
	}
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
