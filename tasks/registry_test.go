package tasks

import (
	"bytes"
	"fmt"
	// "log" // Removed as it's unused
	"os"
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
	fmt.Printf("MockTask %s executed with params: %v\n", m.TaskName, params)
	return nil
}

func resetRegistry() {
	// Clear the existing registry
	taskRegistry = make(map[string]models.Task)
}

// Helper to capture stdout/stderr output
func captureStdOutput(f func()) string {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f() // Execute the function whose output we want to capture

	w.Close()
	os.Stdout = oldStdout // Restore stdout
	var buf bytes.Buffer
	buf.ReadFrom(r)
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
	output := captureStdOutput(func() {
		RegisterTask(task2) // Second registration, should overwrite and warn
	})

	expectedWarning := "Warning: Task with name 'OverwriteTest' is being overwritten in the registry."
	if !strings.Contains(output, expectedWarning) {
		t.Errorf("Expected log output to contain warning '%s', but got: %s", expectedWarning, output)
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

	output := captureStdOutput(func() {
		RegisterTask(task)
	})

	expectedLog := "Task 'SuccessfulRegistrationTest' registered successfully."
	if !strings.Contains(output, expectedLog) {
		t.Errorf("Expected log output to contain success message '%s', but got: %s", expectedLog, output)
	}
}

func TestUnregisterTask(t *testing.T) {
	resetRegistry()
	task := &MockTask{TaskName: "UnregisterTest"}
	RegisterTask(task)

	_, err := GetTask("UnregisterTest")
	if err != nil {
		t.Fatalf("Task should be present before unregistering, but got error: %v", err)
	}

	output := captureStdOutput(func() {
		UnregisterTaskForTesting("UnregisterTest")
	})

	expectedLog := "Task 'UnregisterTest' unregistered for testing."
	if !strings.Contains(output, expectedLog) {
		t.Errorf("Expected log output to contain unregister message '%s', but got: %s", expectedLog, output)
	}

	_, err = GetTask("UnregisterTest")
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
