package models

// Task represents a job that can be scheduled and executed.
// Implementations of this interface define specific actions to be performed.
type Task interface {
	// Name returns the unique identifier for the task.
	// This name is used in the schedule.json to refer to this task.
	Name() string

	// Execute performs the action defined by the task.
	// params provides a way to pass task-specific parameters defined in schedule.json.
	// It returns an error if the execution fails, otherwise nil.
	Execute(params map[string]interface{}) error
}
