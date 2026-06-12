package config

import "testing"

// Batch 5B: DEADLINE_ENGINE_V2_SHADOW must default false so shadow compute
// does not run unless explicitly enabled.
func TestLoad_DeadlineEngineV2ShadowDefaultFalse(t *testing.T) {
	t.Setenv("DEADLINE_ENGINE_V2_SHADOW", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DeadlineEngineV2Shadow {
		t.Fatal("DeadlineEngineV2Shadow must default to false")
	}
}

func TestLoad_DeadlineEngineV2ShadowExplicitTrue(t *testing.T) {
	t.Setenv("DEADLINE_ENGINE_V2_SHADOW", "true")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.DeadlineEngineV2Shadow {
		t.Fatal("DeadlineEngineV2Shadow want true when env set")
	}
}
