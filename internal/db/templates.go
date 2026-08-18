package db

import (
	"database/sql"
	"fmt"
)

// TemplateMeta is the listing view of a stored Z-layout template.
type TemplateMeta struct {
	Name      string `json:"name"`
	UpdatedAt string `json:"updated_at"`
}

// SetupTemplatesTable creates the layout_templates table if it does not exist.
func SetupTemplatesTable(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS layout_templates (
		name          TEXT PRIMARY KEY,
		chromium_yaml TEXT NOT NULL,
		updated_at    DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	if _, err := db.Exec(query); err != nil {
		return fmt.Errorf("failed to create layout_templates table: %w", err)
	}
	return nil
}

// SaveTemplate inserts or replaces a named template's chromium_source YAML.
func SaveTemplate(db *sql.DB, name, chromiumYAML string) error {
	_, err := db.Exec(
		`INSERT INTO layout_templates (name, chromium_yaml, updated_at)
		 VALUES (?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(name) DO UPDATE SET
		   chromium_yaml = excluded.chromium_yaml,
		   updated_at    = CURRENT_TIMESTAMP`,
		name, chromiumYAML)
	if err != nil {
		return fmt.Errorf("failed to save template %q: %w", name, err)
	}
	return nil
}

// ListTemplates returns all templates' metadata, ordered by name.
func ListTemplates(db *sql.DB) ([]TemplateMeta, error) {
	rows, err := db.Query(`SELECT name, updated_at FROM layout_templates ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("failed to list templates: %w", err)
	}
	defer rows.Close()

	out := []TemplateMeta{}
	for rows.Next() {
		var m TemplateMeta
		if err := rows.Scan(&m.Name, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan template row: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// GetTemplate returns a template's stored chromium_source YAML.
func GetTemplate(db *sql.DB, name string) (string, error) {
	var y string
	err := db.QueryRow(`SELECT chromium_yaml FROM layout_templates WHERE name = ?`, name).Scan(&y)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("template %q not found", name)
	}
	if err != nil {
		return "", fmt.Errorf("get template %q: %w", name, err)
	}
	return y, nil
}

// DeleteTemplate removes a template; it is an error if no such template exists.
func DeleteTemplate(db *sql.DB, name string) error {
	res, err := db.Exec(`DELETE FROM layout_templates WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("delete template %q: %w", name, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("template %q not found", name)
	}
	return nil
}
