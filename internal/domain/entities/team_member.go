package entities

import "time"

const (
	TeamRoleOwner  = "owner"
	TeamRoleMember = "member"
)

type TeamMember struct {
	TeamID    int64
	UserID    int64
	Role      string
	CreatedAt time.Time
}
