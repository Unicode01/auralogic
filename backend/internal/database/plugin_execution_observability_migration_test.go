package database

import (
	"testing"
	"time"

	"auralogic/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMigratePluginExecutionObservabilityFieldsTerminatesWithNonHookRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:plugin-observability-migration?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&models.Plugin{}, &models.PluginExecution{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	previousDB := DB
	DB = db
	defer func() {
		DB = previousDB
	}()

	seed := []models.PluginExecution{
		{
			PluginID: 1,
			Action:   "execute",
			Success:  true,
			Params:   `{"foo":"bar"}`,
		},
		{
			PluginID: 1,
			Action:   "hook.execute",
			Success:  true,
			Params:   `{"hook":"order.create.before"}`,
		},
		{
			PluginID: 1,
			Action:   "sync",
			Success:  false,
			Error:    "request 123 failed",
		},
		{
			PluginID: 1,
			Action:   "hook.execute",
			Success:  true,
			Params:   `{"payload":{"foo":"bar"}}`,
		},
	}
	if err := db.Create(&seed).Error; err != nil {
		t.Fatalf("seed plugin executions failed: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- migratePluginExecutionObservabilityFields()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("migration failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("migration did not terminate; likely stuck reprocessing the same plugin execution rows")
	}

	var executions []models.PluginExecution
	if err := db.Order("id ASC").Find(&executions).Error; err != nil {
		t.Fatalf("query migrated executions failed: %v", err)
	}
	if len(executions) != len(seed) {
		t.Fatalf("expected %d executions, got %d", len(seed), len(executions))
	}

	if executions[0].Hook != "" {
		t.Fatalf("expected non-hook execution to keep empty hook, got %q", executions[0].Hook)
	}
	if executions[1].Hook != "order.create.before" {
		t.Fatalf("expected hook to be backfilled, got %q", executions[1].Hook)
	}
	if executions[2].ErrorSignature != "request # failed" {
		t.Fatalf("expected error signature to be backfilled, got %q", executions[2].ErrorSignature)
	}
	if executions[3].Hook != "" {
		t.Fatalf("expected hook to remain empty when payload does not include hook name, got %q", executions[3].Hook)
	}
}
