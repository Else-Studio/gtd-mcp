package sqlite

import (
	"errors"
	"testing"
)

func TestIsInactiveTransaction(t *testing.T) {
	if isInactiveTransaction(nil) {
		t.Fatal("nil should not be inactive")
	}
	if isInactiveTransaction(errors.New("database is locked")) {
		t.Fatal("unrelated error should not match")
	}
	err := errors.New("sqlite3: SQL logic error; cannot commit - no transaction is active")
	if !isInactiveTransaction(err) {
		t.Fatalf("wanted match for %v", err)
	}
}
