package teams

import "tasks-service/internal/domain/entities"

func canManageTeam(actor entities.Actor, team entities.Team) bool {
	if actor.IsAdmin {
		return true
	}

	return team.CreatedBy == actor.UserID
}

func canDeleteTeam(actor entities.Actor, team entities.Team) bool {
	return canManageTeam(actor, team)
}

func canInviteMember(actor entities.Actor, team entities.Team) bool {
	return canManageTeam(actor, team)
}

func canRemoveMember(actor entities.Actor, team entities.Team) bool {
	return canManageTeam(actor, team)
}
