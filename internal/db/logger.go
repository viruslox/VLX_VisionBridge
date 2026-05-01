package db

import (
	"database/sql"
	"fmt"
	"time"
)

// SetupTables creates the necessary tables if they don't exist.
func SetupTables(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS stream_logs (
		id SERIAL PRIMARY KEY,
		event_type VARCHAR(50) NOT NULL,
		message TEXT,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create stream_logs table: %w", err)
	}
	return nil
}

// LogStreamEvent logs a stream event to the database.
func LogStreamEvent(db *sql.DB, eventType, message string) error {
	query := `
	INSERT INTO stream_logs (event_type, message, created_at)
	VALUES ($1, $2, $3)
	`
	_, err := db.Exec(query, eventType, message, time.Now())
	if err != nil {
		return fmt.Errorf("failed to log stream event: %w", err)
	}
	return nil
}
