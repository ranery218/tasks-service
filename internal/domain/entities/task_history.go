package entities

import "time"

type TaskHistory struct {
	ID        int64
	TaskID    int64
	Field     string
	OldValue  string
	NewValue  string
	ChangedBy int64
	CreatedAt time.Time
}
