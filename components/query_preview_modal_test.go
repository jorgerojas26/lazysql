package components

import (
	"errors"
	"testing"

	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/models"
)

func TestRemoveAppliedPendingChanges(t *testing.T) {
	queries := []models.DBDMLChange{
		{Table: "already-applied"},
		{Table: "failed"},
		{Table: "not-attempted"},
	}
	err := &drivers.PartialExecutionError{Applied: 1, Err: errors.New("failed")}

	if !removeAppliedPendingChanges(&queries, err) {
		t.Fatal("expected applied changes to be removed")
	}
	if len(queries) != 2 || queries[0].Table != "failed" || queries[1].Table != "not-attempted" {
		t.Fatalf("unexpected remaining changes: %#v", queries)
	}
}

func TestRemoveAppliedPendingChangesIgnoresRegularErrors(t *testing.T) {
	queries := []models.DBDMLChange{{Table: "still-pending"}}
	if removeAppliedPendingChanges(&queries, errors.New("failed")) {
		t.Fatal("regular errors must not remove pending changes")
	}
	if len(queries) != 1 || queries[0].Table != "still-pending" {
		t.Fatalf("unexpected remaining changes: %#v", queries)
	}
}
