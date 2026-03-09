package entities

type TeamStatsRow struct {
	TeamID             int64
	TeamName           string
	MembersCount       int
	DoneTasksLast7Days int
}

type TopCreatorRow struct {
	TeamID       int64
	UserID       int64
	TasksCreated int
	Rank         int
}

type InvalidAssigneeRow struct {
	TaskID     int64
	TeamID     int64
	AssigneeID int64
}
