package tasks

import (
	"fmt"
	"github.com/c-m3-codin/gsched/models"
)

// Dummy variable to ensure models import is considered used by explicitly
// asserting interface compliance. This also helps catch interface mismatches early.
var _ models.Task = ((*LogTask)(nil))
var _ models.Task = ((*EchoTask)(nil))

// LogTask is an example task that logs a message.
type LogTask struct{}

// Name returns the name of the LogTask.
func (lt *LogTask) Name() string {
	return "LogTask"
}

// Execute prints a message from the task parameters.
// It expects a "message" key in the params map.
func (lt *LogTask) Execute(params map[string]interface{}) error {
	message, ok := params["message"].(string)
	if !ok {
		return fmt.Errorf("LogTask: 'message' parameter is missing or not a string")
	}
	fmt.Printf("LogTask executing: %s\n", message)
	return nil
}

// EchoTask is an example task that prints its parameters.
type EchoTask struct{}

// Name returns the name of the EchoTask.
func (et *EchoTask) Name() string {
	return "EchoTask"
}

// Execute prints all parameters it receives.
func (et *EchoTask) Execute(params map[string]interface{}) error {
	fmt.Println("EchoTask executing with parameters:")
	if len(params) == 0 {
		fmt.Println("  (No parameters received)")
		return nil
	}
	for key, value := range params {
		fmt.Printf("  %s: %v\n", key, value)
	}
	return nil
}

func init() {
	RegisterTask(&LogTask{})
	RegisterTask(&EchoTask{})
}
