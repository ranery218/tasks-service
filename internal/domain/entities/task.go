package entities

import "time"

const (
	TaskStatusTodo       = "todo"
	TaskStatusInProgress = "in_progress"
	TaskStatusDone       = "done"
)

type Task struct {
	ID          int64
	TeamID      int64
	Title       string
	Description string
	AssigneeID  *int64
	CreatedBy   int64
	Status      string
	DueDate     *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type TaskFilter struct {
	TeamID     *int64
	Status     string
	AssigneeID *int64
	Limit      int
	Offset     int
}
