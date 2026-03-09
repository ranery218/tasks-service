package entities

import "time"

type TaskComment struct {
	ID        int64
	TaskID    int64
	UserID    int64
	Text      string
	CreatedAt time.Time
}
