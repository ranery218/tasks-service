package entities

import "time"

type DependencyStatus struct {
	Name    string
	Healthy bool
	Error   string
}

type ReadyStatus struct {
	Status       string
	CheckedAtUTC time.Time
	Dependencies []DependencyStatus
}

type HealthStatus struct {
	Status       string
	CheckedAtUTC time.Time
}
