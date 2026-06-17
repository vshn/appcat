package v1

import "testing"

func TestVSHNForgejoRunnerSpec_GetPlan(t *testing.T) {
	r := &VSHNForgejoRunnerSpec{}
	if got := r.GetPlan("runner-mini"); got != "runner-mini" {
		t.Fatalf("expected default plan runner-mini, got %q", got)
	}

	r.Size.Plan = "runner-large"
	if got := r.GetPlan("runner-mini"); got != "runner-large" {
		t.Fatalf("expected explicit plan runner-large, got %q", got)
	}
}
