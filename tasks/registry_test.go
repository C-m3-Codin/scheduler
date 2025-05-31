package tasks

import (
	"bytes"
	"fmt"
	"log/slog" // New import for slog
	// "os"    // No longer needed after removing captureStdOutput
	"strings"
	"testing"

	"github.com/c-m3-codin/gsched/models"
)

// MockTask is a simple mock for testing.
type MockTask struct {
	TaskName    string
	ExecuteFunc func(params map[string]interface{}) error
}

func (m *MockTask) Name() string {
	return m.TaskName
}

func (m *MockTask) Execute(params map[string]interface{}) error {
	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(params)
	}
	// Keep a simple fmt.Printf here for direct test feedback if needed,
	// but this task's execution isn't the primary focus of registry tests.
	// fmt.Printf("MockTask %s executed with params: %v\n", m.TaskName, params)
	return nil
}

func resetRegistry() {
	// Clear the existing registry
	taskRegistry = make(map[string]models.Task)
	// It's also important to reset the slog default handler if tests modify it,
	// but captureSlogOutput handles this for its scope.
}

// Helper to capture slog output for testing warnings/debug messages
func captureSlogOutput(fn func()) string {
	var buf bytes.Buffer
	// Create a new handler writing to the local buffer
	// Using a simple text handler for easier string matching in these tests,
	// assuming the global logger might be JSON. Or use JSON and match parts.
	// For consistency with global logger, let's assume JSON and match key parts.
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug, AddSource: false}) // AddSource:false for simpler logs in test
	// Store the default logger
	oldLogger := slog.Default()
	// Set our new logger as default
	slog.SetDefault(slog.New(handler))
	// Ensure the original logger is restored after the function call
	defer slog.SetDefault(oldLogger)

	fn() // Execute the function that should generate log output
	return buf.String()
}

func TestRegisterAndGetTask(t *testing.T) {
	resetRegistry() // Ensure clean state
	mockTask := &MockTask{TaskName: "TestTask1"}
	RegisterTask(mockTask)

	retrievedTask, err := GetTask("TestTask1")
	if err != nil {
		t.Fatalf("Expected to get task 'TestTask1', but got error: %v", err)
	}
	if retrievedTask.Name() != "TestTask1" {
		t.Errorf("Expected task name 'TestTask1', but got '%s'", retrievedTask.Name())
	}
}

func TestGetNonExistentTask(t *testing.T) {
	resetRegistry() // Ensure clean state
	_, err := GetTask("NonExistentTask")
	if err == nil {
		t.Fatalf("Expected an error when getting a non-existent task, but got nil")
	}
	expectedErrorMsg := "task with name 'NonExistentTask' not found in registry"
	if err.Error() != expectedErrorMsg {
		t.Errorf("Expected error message '%s', but got '%s'", expectedErrorMsg, err.Error())
	}
}

func TestOverwriteTask(t *testing.T) {
	resetRegistry() // Ensure clean state
	task1 := &MockTask{TaskName: "OverwriteTest"}
	task2 := &MockTask{TaskName: "OverwriteTest", ExecuteFunc: func(params map[string]interface{}) error { return fmt.Errorf("task2 executed") }}

	RegisterTask(task1) // First registration

	// Capture output during the second registration to check for the warning
	logOutput := captureSlogOutput(func() {
		RegisterTask(task2) // Second registration, should overwrite and warn
	})

	// Check for structured log parts for the warning
	if !strings.Contains(logOutput, `"level":"WARN"`) ||
		!strings.Contains(logOutput, `"msg":"Task being overwritten in registry"`) ||
		!strings.Contains(logOutput, `"task_name":"OverwriteTest"`) {
		t.Errorf("Expected warning log for task overwrite not found or incorrect. Log: %s", logOutput)
	}

	retrievedTask, err := GetTask("OverwriteTest")
	if err != nil {
		t.Fatalf("Expected to get task 'OverwriteTest' after overwrite, but got error: %v", err)
	}

	// Check if it's task2 by trying to execute and see if we get task2's error
	err = retrievedTask.Execute(nil)
	if err == nil || err.Error() != "task2 executed" {
		t.Errorf("Expected task to be overwritten with task2. Execute() should return task2's specific error. Got: %v", err)
	}
}

func TestRegisterTaskLogsSuccess(t *testing.T) {
	resetRegistry()
	task := &MockTask{TaskName: "SuccessfulRegistrationTest"}

	logOutput := captureSlogOutput(func() {
		RegisterTask(task)
	})

	if !strings.Contains(logOutput, `"level":"INFO"`) ||
		!strings.Contains(logOutput, `"msg":"Task registered successfully"`) ||
		!strings.Contains(logOutput, `"task_name":"SuccessfulRegistrationTest"`) {
		t.Errorf("Expected info log for successful task registration not found or incorrect. Log: %s", logOutput)
	}
}

func TestUnregisterTask(t *testing.T) {
	resetRegistry()
	task := &MockTask{TaskName: "UnregisterTest"}
	RegisterTask(task) // This will produce INFO log, not captured by this test's logOutput

	_, err := GetTask("UnregisterTest")
	if err != nil {
		t.Fatalf("Task should be present before unregistering, but got error: %v", err)
	}

	// Capture the output specifically for UnregisterTaskForTesting
	logOutput := captureSlogOutput(func() {
		UnregisterTaskForTesting("UnregisterTest")
	})

	if !strings.Contains(logOutput, `"level":"DEBUG"`) ||
		!strings.Contains(logOutput, `"msg":"Task unregistered for testing"`) ||
		!strings.Contains(logOutput, `"task_name":"UnregisterTest"`) {
		t.Errorf("Expected debug log for task unregistration not found or incorrect. Log: %s", logOutput)
	}

	_, err = GetTask("UnregisterTest") // Re-check err after getting the task
	if err == nil {
		t.Errorf("Expected error after unregistering task, but GetTask succeeded.")
	}
	expectedErrorMsg := "task with name 'UnregisterTest' not found in registry"
	if err == nil || err.Error() != expectedErrorMsg {
		t.Errorf("Expected error message '%s' after unregister, but got: %v", expectedErrorMsg, err)
	}
}
// Note: The original prompt mentioned `log.SetOutput` for capturing logs.
// However, RegisterTask uses `fmt.Printf`. So, a more general stdout capture is used here.
// If RegisterTask were changed to use the `log` package, then `log.SetOutput` would be appropriate.
// The `captureStdOutput` helper is designed to capture `fmt.Printf` output to `os.Stdout`.
// The `resetRegistry` function was also slightly modified to ensure it re-initializes the map.
// Added TestRegisterTaskLogsSuccess and TestUnregisterTask for completeness.
