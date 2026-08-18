package db

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	err = SetupTemplatesTable(db)
	require.NoError(t, err)

	return db
}

func TestTemplatesCRUD(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// List empty
	list, err := ListTemplates(db)
	require.NoError(t, err)
	assert.Empty(t, list)

	// Save template A
	err = SaveTemplate(db, "templateA", "chromium_source: A")
	require.NoError(t, err)

	// Get template A
	yaml, err := GetTemplate(db, "templateA")
	require.NoError(t, err)
	assert.Equal(t, "chromium_source: A", yaml)

	// Save template B
	err = SaveTemplate(db, "templateB", "chromium_source: B")
	require.NoError(t, err)

	// List orders by name
	list, err = ListTemplates(db)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, "templateA", list[0].Name)
	assert.Equal(t, "templateB", list[1].Name)

	// Upsert template A
	err = SaveTemplate(db, "templateA", "chromium_source: A_v2")
	require.NoError(t, err)

	yaml, err = GetTemplate(db, "templateA")
	require.NoError(t, err)
	assert.Equal(t, "chromium_source: A_v2", yaml)

	// Delete template A
	err = DeleteTemplate(db, "templateA")
	require.NoError(t, err)

	// Get missing
	_, err = GetTemplate(db, "templateA")
	assert.Error(t, err)

	// Delete missing
	err = DeleteTemplate(db, "templateA")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Verify only B remains
	list, err = ListTemplates(db)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "templateB", list[0].Name)
}
