package config

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings" // Import strings for bytes.Contains with string literals
	"testing"

	"github.com/c-m3-codin/gsched/constants"
)

// Helper to capture slog output for testing warnings/debug messages
func captureSlogOutput(fn func()) string {
	var buf bytes.Buffer
	// Create a new handler writing to the local buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug, AddSource: true})
	// Store the default logger
	oldLogger := slog.Default()
	// Set our new logger as default
	slog.SetDefault(slog.New(handler))
	// Ensure the original logger is restored after the function call
	defer slog.SetDefault(oldLogger)

	fn() // Execute the function that should generate log output
	return buf.String()
}

func TestLoadConfig_FileFoundAndParsed(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "kafka.properties")
	content := []byte("bootstrap.servers=test.broker:9092\njob.topic=TestTopic\nconsumer.group.id=TestGroup")
	if err := os.WriteFile(tempFile, content, 0644); err != nil {
		t.Fatalf("Failed to write temp config file: %v", err)
	}

	cfg := LoadConfig(tempFile)

	if cfg.BootstrapServers != "test.broker:9092" {
		t.Errorf("Expected BootstrapServers 'test.broker:9092', got '%s'", cfg.BootstrapServers)
	}
	if cfg.JobTopic != "TestTopic" {
		t.Errorf("Expected JobTopic 'TestTopic', got '%s'", cfg.JobTopic)
	}
	if cfg.ConsumerGroupID != "TestGroup" {
		t.Errorf("Expected ConsumerGroupID 'TestGroup', got '%s'", cfg.ConsumerGroupID)
	}
}

func TestLoadConfig_FileNotFound_UsesDefaults(t *testing.T) {
	nonExistentFile := filepath.Join(t.TempDir(), "nonexistent.properties")
	var cfg AppConfig // Declare cfg outside to check its value

	logOutput := captureSlogOutput(func() {
		cfg = LoadConfig(nonExistentFile) // Assign to the outer cfg

		if cfg.BootstrapServers != constants.DefaultKafkaBootstrapServers {
			t.Errorf("Expected BootstrapServers default '%s', got '%s'", constants.DefaultKafkaBootstrapServers, cfg.BootstrapServers)
		}
		if cfg.JobTopic != constants.DefaultJobTopic {
			t.Errorf("Expected JobTopic default '%s', got '%s'", constants.DefaultJobTopic, cfg.JobTopic)
		}
		if cfg.ConsumerGroupID != constants.DefaultConsumerGroupID {
			t.Errorf("Expected ConsumerGroupID default '%s', got '%s'", constants.DefaultConsumerGroupID, cfg.ConsumerGroupID)
		}
	})

	// Check if warning was logged
	// Using strings.Contains for simplicity with JSON log output.
	// For more robust checks, one might parse the JSON log entries.
	if !strings.Contains(logOutput, "Failed to load config file") {
		t.Errorf("Expected warning about missing config file, but not found in logs: %s", logOutput)
	}
	if !strings.Contains(logOutput, nonExistentFile) { // Check if the path was logged
		t.Errorf("Expected missing file path '%s' in logs, but not found: %s", nonExistentFile, logOutput)
	}
}

func TestLoadConfig_PartialFile_UsesDefaultsForMissingKeys(t *testing.T) {
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "kafka_partial.properties")
	// Missing job.topic and consumer.group.id
	content := []byte("bootstrap.servers=partial.broker:9092\n#job.topic is missing\n  some.other.key = some.value")
	if err := os.WriteFile(tempFile, content, 0644); err != nil {
		t.Fatalf("Failed to write temp partial config file: %v", err)
	}
	var cfg AppConfig

	logOutput := captureSlogOutput(func() {
		cfg = LoadConfig(tempFile)

		if cfg.BootstrapServers != "partial.broker:9092" {
			t.Errorf("Expected BootstrapServers 'partial.broker:9092', got '%s'", cfg.BootstrapServers)
		}
		if cfg.JobTopic != constants.DefaultJobTopic { // Should fallback
			t.Errorf("Expected JobTopic default '%s' due to missing key, got '%s'", constants.DefaultJobTopic, cfg.JobTopic)
		}
		if cfg.ConsumerGroupID != constants.DefaultConsumerGroupID { // Should fallback
			t.Errorf("Expected ConsumerGroupID default '%s' due to missing key, got '%s'", constants.DefaultConsumerGroupID, cfg.ConsumerGroupID)
		}
	})

	if !strings.Contains(logOutput, "Config key not found or empty, using default") ||
		!strings.Contains(logOutput, "\"key\":\"job.topic\"") ||
		!strings.Contains(logOutput, "\"key\":\"consumer.group.id\"") {
		t.Errorf("Expected debug messages about using defaults for missing keys ('job.topic', 'consumer.group.id'), but not found or incomplete in logs: %s", logOutput)
	}
}

func TestLoadConfig_EmptyValues_UsesDefaults(t *testing.T) {
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "kafka_empty_values.properties")
	// All keys present but with empty values
	content := []byte("bootstrap.servers=\njob.topic=\nconsumer.group.id=")
	if err := os.WriteFile(tempFile, content, 0644); err != nil {
		t.Fatalf("Failed to write temp empty values config file: %v", err)
	}
	var cfg AppConfig

	logOutput := captureSlogOutput(func() {
		cfg = LoadConfig(tempFile)

		if cfg.BootstrapServers != constants.DefaultKafkaBootstrapServers {
			t.Errorf("Expected BootstrapServers default for empty value, got '%s'", cfg.BootstrapServers)
		}
		if cfg.JobTopic != constants.DefaultJobTopic {
			t.Errorf("Expected JobTopic default for empty value, got '%s'", cfg.JobTopic)
		}
		if cfg.ConsumerGroupID != constants.DefaultConsumerGroupID {
			t.Errorf("Expected ConsumerGroupID default for empty value, got '%s'", cfg.ConsumerGroupID)
		}
	})

	// Check that debug messages for fallback due to empty value were logged
	if !strings.Contains(logOutput, "Config key not found or empty, using default") ||
		!strings.Contains(logOutput, "\"key\":\"bootstrap.servers\"") ||
		!strings.Contains(logOutput, "\"key\":\"job.topic\"") ||
		!strings.Contains(logOutput, "\"key\":\"consumer.group.id\"") {
		t.Errorf("Expected debug messages about using defaults for empty values, but not found or incomplete in logs: %s", logOutput)
	}
}

func TestLoadKafkaProperties_CommentsAndEmptyLines(t *testing.T) {
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "kafka_with_comments.properties")
	content := []byte(
		"# This is a comment\n" +
			"bootstrap.servers=comment.broker:9092\n" +
			"\n" + // Empty line
			"job.topic=CommentTopic\n" +
			"  # Indented comment\n" +
			"consumer.group.id = CommentGroup # Trailing comment\n" +
			"malformed.line\n" + // Line without '='
			"=onlyvalue\n")     // Line starting with '='
	if err := os.WriteFile(tempFile, content, 0644); err != nil {
		t.Fatalf("Failed to write temp config file with comments: %v", err)
	}

	props, err := loadKafkaProperties(tempFile)
	if err != nil {
		t.Fatalf("loadKafkaProperties failed: %v", err)
	}

	if val, ok := props["bootstrap.servers"]; !ok || val != "comment.broker:9092" {
		t.Errorf("Expected bootstrap.servers 'comment.broker:9092', got '%v'", props["bootstrap.servers"])
	}
	if val, ok := props["job.topic"]; !ok || val != "CommentTopic" {
		t.Errorf("Expected job.topic 'CommentTopic', got '%v'", props["job.topic"])
	}
	if val, ok := props["consumer.group.id"]; !ok || val != "CommentGroup" { // Value should be trimmed of trailing comment
		t.Errorf("Expected consumer.group.id 'CommentGroup', got '%v'", props["consumer.group.id"])
	}
    if _, ok := props["# This is a comment"]; ok {
        t.Error("Comment line should not be parsed as a key")
    }
    if _, ok := props["malformed.line"]; ok {
        t.Error("Malformed line should not be parsed as a key")
    }
    if _, ok := props[""]; ok {
         // Test for key being "" from "=onlyvalue"
        t.Error("Line starting with '=' resulting in empty key should not be parsed or handled gracefully")
    }
     // Check that the map only contains the expected keys
    if len(props) != 3 {
        t.Errorf("Expected 3 properties, got %d. Props: %v", len(props), props)
    }
}
