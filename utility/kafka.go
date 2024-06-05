package utility

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/c-m3-codin/gsched/models"
	"github.com/confluentinc/confluent-kafka-go/kafka"
)

var producer *kafka.Producer

func init() {
	// Initialize the Kafka producer client

	fmt.Println("\n\n\n\n\n init in kafka worked \n\n\n\n\n ")
	config := &kafka.ConfigMap{
		"bootstrap.servers":       "broker:9092,broker-2:9093",
		"broker.version.fallback": "0.10.0.0", // Fallback broker version, for older brokers
		"api.version.request":     true,       //
	}

	p, err := kafka.NewProducer(config)
	topic := "JobQueue2"
	fmt.Println(p.GetMetadata(&topic, true, 10))
	if err != nil {
		fmt.Printf("\n\n\n Error creating Kafka producer: %v\n", err)
		return
	}

	producer = p

	PushToQueue(models.Job{JobName: "test", Priority: 4})
}

func closeKafka() {
	producer.Close()

}

func PushToQueue(job models.Job) {
	topic := "JobQueue"

	// Go-routine to handle message delivery reports and
	// possibly other event types (errors, stats, etc)
	go func() {
		for e := range producer.Events() {
			switch ev := e.(type) {
			case *kafka.Message:
				if ev.TopicPartition.Error != nil {
					fmt.Printf("Failed to deliver message: %v\n", ev.TopicPartition)
				} else {
					fmt.Printf("Produced event to topic %s: key = %-10s value = %s\n",
						*ev.TopicPartition.Topic, string(ev.Key), string(ev.Value))
				}
			}
		}
	}()

	key := strconv.Itoa(job.Priority)
	data, err := json.Marshal(job)
	if err != nil {
		fmt.Printf("Couldnt marshall the job details")
	}

	producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Key:            []byte(key),
		Value:          data,
	}, nil)

	// Wait for all messages to be delivered
	producer.Flush(15 * 1000)

}

func ReadConfig(configFile string) kafka.ConfigMap {

	m := make(map[string]kafka.ConfigValue)

	file, err := os.Open(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open file: %s", err)
		os.Exit(1)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "#") && len(line) != 0 {
			before, after, found := strings.Cut(line, "=")
			if found {
				parameter := strings.TrimSpace(before)
				value := strings.TrimSpace(after)
				m[parameter] = value
			}
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Failed to read file: %s", err)
		os.Exit(1)
	}

	return m

}
