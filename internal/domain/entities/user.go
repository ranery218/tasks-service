package entities

import "time"

type User struct {
	ID           int64
	Email        string
	PasswordHash string
	Name         string
	IsAdmin      bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
