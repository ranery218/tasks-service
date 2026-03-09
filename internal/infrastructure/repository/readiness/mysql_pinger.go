package readiness

import (
	"context"
	"database/sql"
)

type MySQLPinger struct {
	db *sql.DB
}

func NewMySQLPinger(db *sql.DB) MySQLPinger {
	return MySQLPinger{db: db}
}

func (p MySQLPinger) Name() string {
	return "mysql"
}

func (p MySQLPinger) Ping(ctx context.Context) error {
	return p.db.PingContext(ctx)
}
