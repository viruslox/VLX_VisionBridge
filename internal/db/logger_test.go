package db

import (
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestLogStreamEvent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	eventType := "test_event"
	message := "test message"

	mock.ExpectExec("INSERT INTO stream_logs").
		WithArgs(eventType, message, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = LogStreamEvent(db, eventType, message)
	if err != nil {
		t.Errorf("expected no error, but got: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestLogStreamEvent_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	eventType := "test_event"
	message := "test message"

	expectedErr := fmt.Errorf("some database error")

	mock.ExpectExec("INSERT INTO stream_logs").
		WithArgs(eventType, message, sqlmock.AnyArg()).
		WillReturnError(expectedErr)

	err = LogStreamEvent(db, eventType, message)
	if err == nil {
		t.Error("expected an error, but got nil")
	}

	if err != nil {
		expectedErrStr := fmt.Sprintf("failed to log stream event: %v", expectedErr)
		if err.Error() != expectedErrStr {
			t.Errorf("expected error %q, but got: %q", expectedErrStr, err.Error())
		}
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestSetupTables(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS stream_logs").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = SetupTables(db)
	if err != nil {
		t.Errorf("expected no error, but got: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestSetupTables_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	expectedErr := fmt.Errorf("some database error")

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS stream_logs").
		WillReturnError(expectedErr)

	err = SetupTables(db)
	if err == nil {
		t.Error("expected an error, but got nil")
	}

	if err != nil {
		expectedErrStr := fmt.Sprintf("failed to create stream_logs table: %v", expectedErr)
		if err.Error() != expectedErrStr {
			t.Errorf("expected error %q, but got: %q", expectedErrStr, err.Error())
		}
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
