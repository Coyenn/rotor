package conformance

import "testing"

func TestBehavioralSuite(t *testing.T) {
	tools := detectRuntimeTools()
	if tools.Rojo == "" || tools.Lune == "" {
		t.Skipf("runtime suite requires rojo and lune; rojo=%q lune=%q", tools.Rojo, tools.Lune)
	}
	if err := runBehavioralSuite(repoRoot(t)); err != nil {
		t.Fatal(err)
	}
}
