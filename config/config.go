package config

import (
	"bufio"
	"log/slog"
	"os"
	"strings"

	"github.com/c-m3-codin/gsched/constants"
)

type AppConfig struct {
	BootstrapServers string
	JobTopic         string
	ConsumerGroupID  string
}

// loadKafkaProperties reads a properties file (key=value format) and returns a map.
func loadKafkaProperties(filePath string) (map[string]string, error) {
	props := make(map[string]string)
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err // Return error to indicate file not found or unreadable
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		line = strings.SplitN(line, "#", 2)[0] // Remove comments from the line
		line = strings.TrimSpace(line)
		if len(line) == 0 { // Skip empty lines (after comment removal)
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if key != "" { // Ensure key is not empty
				props[key] = value
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return props, nil
}

func LoadConfig(filePath string) AppConfig {
	props, err := loadKafkaProperties(filePath)
	if err != nil {
		slog.Warn("Failed to load config file, using default Kafka settings", "path", filePath, "error", err)
		// Fallback to defaults if file not found or error reading
		return AppConfig{
			BootstrapServers: constants.DefaultKafkaBootstrapServers,
			JobTopic:         constants.DefaultJobTopic,
			ConsumerGroupID:  constants.DefaultConsumerGroupID,
		}
	}

	getStringOrDefault := func(key string, defaultValue string) string {
		if val, ok := props[key]; ok && val != "" {
			return val
		}
		slog.Debug("Config key not found or empty, using default", "key", key, "default_value", defaultValue)
		return defaultValue
	}

	return AppConfig{
		BootstrapServers: getStringOrDefault("bootstrap.servers", constants.DefaultKafkaBootstrapServers),
		JobTopic:         getStringOrDefault("job.topic", constants.DefaultJobTopic),
		ConsumerGroupID:  getStringOrDefault("consumer.group.id", constants.DefaultConsumerGroupID),
	}
}
