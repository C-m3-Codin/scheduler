package tasks

import (
	"fmt"
	"sync"

	"github.com/c-m3-codin/gsched/models"
)

var (
	taskRegistry = make(map[string]models.Task)
	registryLock = &sync.RWMutex{}
)

// RegisterTask adds a task to the central registry.
// If a task with the same name already exists, it will be overwritten
// and a warning will be logged.
func RegisterTask(task models.Task) {
	registryLock.Lock()
	defer registryLock.Unlock()

	name := task.Name()
	if _, exists := taskRegistry[name]; exists {
		fmt.Printf("Warning: Task with name '%s' is being overwritten in the registry.\n", name)
	}
	taskRegistry[name] = task
	fmt.Printf("Task '%s' registered successfully.\n", name)
}

// GetTask retrieves a task from the registry by its name.
// It returns the task if found, or an error if no task with that name exists.
func GetTask(name string) (models.Task, error) {
	registryLock.RLock()
	defer registryLock.RUnlock()

	task, exists := taskRegistry[name]
	if !exists {
		return nil, fmt.Errorf("task with name '%s' not found in registry", name)
	}
	return task, nil
}

// UnregisterTaskForTesting removes a task from the registry.
// This is intended for use in tests to ensure a clean state.
func UnregisterTaskForTesting(name string) {
	registryLock.Lock()
	defer registryLock.Unlock()
	delete(taskRegistry, name)
	fmt.Printf("Task '%s' unregistered for testing.\n", name)
}
