package repository

import (
	"strings"
	"testing"

	"github.com/josebianco/go-vue-data-lab/internal/domain/models"
)

func TestNewJobExecutionRepository(t *testing.T) {
	db := dryRunDB(t)
	if repo := NewJobExecutionRepository(db); repo == nil {
		t.Fatal("expected non-nil repository")
	}
}

func TestJobExecutionListQuery(t *testing.T) {
	db := dryRunDB(t)

	stmt := jobExecutionListQuery(db, ListOptions{Limit: 20, Offset: 40}).
		Find(&[]models.JobExecution{}).Statement
	sql := stmt.SQL.String()

	for _, fragment := range []string{"ORDER BY started_at DESC", "id DESC", "LIMIT", "OFFSET"} {
		if !strings.Contains(sql, fragment) {
			t.Errorf("expected %q in %s", fragment, sql)
		}
	}
}

func TestNewCronScheduleRepository(t *testing.T) {
	db := dryRunDB(t)
	if repo := NewCronScheduleRepository(db); repo == nil {
		t.Fatal("expected non-nil repository")
	}
}

func TestCronScheduleListQuery(t *testing.T) {
	db := dryRunDB(t)

	stmt := cronScheduleListQuery(db).Find(&[]models.CronSchedule{}).Statement
	sql := stmt.SQL.String()
	if !strings.Contains(sql, "ORDER BY name ASC") {
		t.Errorf("expected name ordering, got %s", sql)
	}
}
