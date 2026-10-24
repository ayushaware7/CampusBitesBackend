// Package database opens the MySQL connection pool.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
)

// Open connects and pings. multiStatements is only for the migration tool: it lets one
// migration file hold several statements, and must stay off for the serving app.
func Open(dsn string, multiStatements bool) (*sql.DB, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("bad DB_DSN: %w", err)
	}
	cfg.ParseTime = true
	// Make RowsAffected count matched rows, not changed rows, so an UPDATE that rewrites
	// identical values is not mistaken for "row not found".
	cfg.ClientFoundRows = true
	cfg.MultiStatements = multiStatements
	// Read and write TIMESTAMPs in UTC regardless of the MySQL server's own time zone, so
	// created_at values are not shifted by the server offset when parsed.
	if cfg.Params == nil {
		cfg.Params = map[string]string{}
	}
	cfg.Params["time_zone"] = "'+00:00'"

	connector, err := mysql.NewConnector(cfg)
	if err != nil {
		return nil, err
	}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("cannot connect to database: %w", err)
	}
	return db, nil
}
