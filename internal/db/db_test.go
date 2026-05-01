package db

import (
	"fmt"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestInitDB_Success(t *testing.T) {
	// Temporarily override driver name to use sqlmock
	originalDriver := driverName
	driverName = "sqlmock"
	defer func() { driverName = originalDriver }()

	// Create a mock DB with a specific DSN
	_, mock, err := sqlmock.NewWithDSN("mock_dsn", sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	// Expect a ping
	mock.ExpectPing()

	// Call InitDB with the mock DSN
	db, err := InitDB("mock_dsn")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if db == nil {
		t.Fatal("expected db connection, got nil")
	}
	defer db.Close()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestInitDB_PingError(t *testing.T) {
	// Temporarily override driver name to use sqlmock
	originalDriver := driverName
	driverName = "sqlmock"
	defer func() { driverName = originalDriver }()

	// Create a mock DB with a specific DSN
	_, mock, err := sqlmock.NewWithDSN("mock_dsn_err", sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	// Expect a ping and return an error
	mock.ExpectPing().WillReturnError(fmt.Errorf("mock ping error"))

	// Call InitDB with the mock DSN
	db, err := InitDB("mock_dsn_err")
	if err == nil {
		t.Error("expected error, got nil")
	}
	if db != nil {
		t.Error("expected db to be nil on error")
	}
	if err != nil && !strings.Contains(err.Error(), "failed to ping database") {
		t.Errorf("expected error to contain 'failed to ping database', got: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestInitDB_OpenError(t *testing.T) {
	// We can test an invalid driver name
	originalDriver := driverName
	driverName = "invalid_driver"
	defer func() { driverName = originalDriver }()

	db, err := InitDB("some_dsn")
	if err == nil {
		t.Error("expected error, got nil")
	}
	if db != nil {
		t.Error("expected db to be nil on error")
	}
	if err != nil && !strings.Contains(err.Error(), "failed to open database") {
		t.Errorf("expected error to contain 'failed to open database', got: %v", err)
	}
}
