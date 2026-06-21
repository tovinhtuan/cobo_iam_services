package config

import "testing"

// Locks the safe default and env parsing for WORKFLOW_ASSIGNEE_RESOLUTION_ENABLED (Sprint 1/Batch 1).
func TestLoad_WorkflowAssigneeResolutionFlag(t *testing.T) {
	t.Run("default_false", func(t *testing.T) {
		t.Setenv("WORKFLOW_ASSIGNEE_RESOLUTION_ENABLED", "")
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.WorkflowAssigneeResolutionEnabled {
			t.Fatal("WorkflowAssigneeResolutionEnabled must default to false")
		}
	})

	t.Run("env_true", func(t *testing.T) {
		t.Setenv("WORKFLOW_ASSIGNEE_RESOLUTION_ENABLED", "true")
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if !cfg.WorkflowAssigneeResolutionEnabled {
			t.Fatal("WorkflowAssigneeResolutionEnabled want true when env=true")
		}
	})
}

// Locks the safe default and env parsing for WORKFLOW_VERSIONING_ENABLED (Sprint 1/Batch 2).
func TestLoad_WorkflowVersioningFlag(t *testing.T) {
	t.Run("default_false", func(t *testing.T) {
		t.Setenv("WORKFLOW_VERSIONING_ENABLED", "")
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.WorkflowVersioningEnabled {
			t.Fatal("WorkflowVersioningEnabled must default to false")
		}
	})
	t.Run("env_true", func(t *testing.T) {
		t.Setenv("WORKFLOW_VERSIONING_ENABLED", "true")
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if !cfg.WorkflowVersioningEnabled {
			t.Fatal("WorkflowVersioningEnabled want true when env=true")
		}
	})
}
