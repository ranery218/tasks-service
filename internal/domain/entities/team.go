package entities

import "time"

type Team struct {
	ID          int64
	Name        string
	Description string
	CreatedBy   int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
