package utility

import (
	"encoding/json"
	"log/slog"
	// "os" // os may not be needed here anymore
	// "bufio" // bufio may not be needed here anymore
	// "strings" // strings may not be needed here anymore
	"strconv"
	// "time" // Removed as producer.Flush is commented out

	"github.com/c-m3-codin/gsched/models"
	"github.com/confluentinc/confluent-kafka-go/kafka" // Using v1 as per go.mod
)

// init function is now empty as per requirements.
func init() {
	slog.Debug("utility/kafka.go init() called (now empty)")
}

// PushToQueue sends a job to the specified Kafka topic using the provided producer.
func PushToQueue(producer *kafka.Producer, topic string, job models.Job) {
	deliveryChan := make(chan kafka.Event)
	// defer close(deliveryChan) // Closing channel explicitly after read

	key := strconv.Itoa(job.Priority)
	data, err := json.Marshal(job)
	if err != nil {
		slog.Error("Failed to marshal job details for Kafka", "job_name", job.JobName, "error", err)
		close(deliveryChan) // Ensure channel is closed on early exit
		return
	}

	err = producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Key:            []byte(key),
		Value:          data,
	}, deliveryChan)

	if err != nil {
		slog.Error("Failed to produce message to Kafka", "topic", topic, "job_name", job.JobName, "error", err)
		close(deliveryChan) // Ensure channel is closed on early exit
		return
	}

	// Wait for delivery report
	e := <-deliveryChan
	close(deliveryChan) // Close channel after reading the event

	m := e.(*kafka.Message)

	if m.TopicPartition.Error != nil {
		slog.Error("Failed to deliver message to Kafka",
			"topic", *m.TopicPartition.Topic,
			"partition", m.TopicPartition.Partition,
			"offset", m.TopicPartition.Offset.String(),
			"error", m.TopicPartition.Error)
	} else {
		slog.Debug("Produced message to Kafka topic",
			"topic", *m.TopicPartition.Topic,
			"key", string(m.Key),
			"partition", m.TopicPartition.Partition,
			"offset", m.TopicPartition.Offset.String())
	}
	// producer.Flush(15 * 1000) // Flushing might be better handled by the caller who manages the producer lifecycle.
}

// NewProducer creates and returns a new Kafka producer instance.
func NewProducer(bootstrapServers string) (*kafka.Producer, error) {
	config := &kafka.ConfigMap{"bootstrap.servers": bootstrapServers}
	p, err := kafka.NewProducer(config)
	if err != nil {
		slog.Error("Failed to create Kafka producer", "bootstrap_servers", bootstrapServers, "error", err)
		return nil, err
	}
	slog.Info("Kafka producer created successfully", "bootstrap_servers", bootstrapServers)

	// Optional: Goroutine to handle delivery reports from the producer's event channel.
	// This is useful for global error handling or logging for the producer instance.
	// If uncommented, ensure it's managed correctly (e.g., closed when producer is closed).
	// go func() {
	// 	for e := range p.Events() {
	// 		switch ev := e.(type) {
	// 		case *kafka.Message:
	// 			if ev.TopicPartition.Error != nil {
	// 				slog.Error("Default delivery report: Failed to deliver message",
	//                            "error", ev.TopicPartition.Error,
	//                            "topic", *ev.TopicPartition.Topic)
	// 			} else {
	// 				slog.Debug("Default delivery report: Message delivered",
	//                            "topic", *ev.TopicPartition.Topic,
	//                            "partition", ev.TopicPartition.Partition,
	//                            "offset", ev.TopicPartition.Offset)
	// 			}
	// 		case kafka.Error:
	// 			slog.Error("Kafka producer error event", "code", ev.Code(), "error", ev.Error())
	// 		default:
	// 			slog.Debug("Kafka producer event ignored", "event_type", ev.String())
	// 		}
	// 	}
	// }()

	return p, nil
}

// NewConsumer creates and returns a new Kafka consumer instance subscribed to the given topic.
func NewConsumer(bootstrapServers string, groupID string, topic string) (*kafka.Consumer, error) {
	config := &kafka.ConfigMap{
		"bootstrap.servers": bootstrapServers,
		"group.id":          groupID,
		"auto.offset.reset": "earliest", // Or "latest" depending on requirements
		// "enable.auto.commit": false, // Consider manual offset commits for more control
	}
	c, err := kafka.NewConsumer(config)
	if err != nil {
		slog.Error("Failed to create Kafka consumer",
			"bootstrap_servers", bootstrapServers,
			"group_id", groupID,
			"error", err)
		return nil, err
	}

	err = c.Subscribe(topic, nil)
	if err != nil {
		c.Close() // Close consumer if subscribe fails
		slog.Error("Failed to subscribe to Kafka topic", "topic", topic, "error", err)
		return nil, err
	}
	slog.Info("Kafka consumer created and subscribed successfully",
		"bootstrap_servers", bootstrapServers,
		"group_id", groupID,
		"topic", topic)
	return c, nil
}

// ReadConfig function removed as its logic is adapted in config/config.go
