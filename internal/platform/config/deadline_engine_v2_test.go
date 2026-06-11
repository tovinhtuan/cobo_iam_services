package config

import "testing"

// MT-12: DEADLINE_ENGINE_V2 must default false so Batch 1 does not change runtime.
func TestLoad_DeadlineEngineV2DefaultFalse(t *testing.T) {
	t.Setenv("DEADLINE_ENGINE_V2", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DeadlineEngineV2 {
		t.Fatal("DeadlineEngineV2 must default to false")
	}
}

func TestLoad_DeadlineEngineV2ExplicitTrue(t *testing.T) {
	t.Setenv("DEADLINE_ENGINE_V2", "true")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.DeadlineEngineV2 {
		t.Fatal("DeadlineEngineV2 want true when env set")
	}
}
