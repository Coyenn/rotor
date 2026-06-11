package term

import "testing"

func TestSparkScalesToMax(t *testing.T) {
	s := &Styler{color: true}
	got := s.Spark([]float64{0, 50, 100})
	if got != "▁▅█" {
		t.Errorf("Spark = %q, want %q", got, "▁▅█")
	}
}

func TestSparkASCIIFallback(t *testing.T) {
	s := &Styler{color: false}
	got := s.Spark([]float64{0, 100})
	if got != ".%" {
		t.Errorf("Spark = %q, want %q", got, ".%")
	}
}

func TestSparkAllZeroAndEmpty(t *testing.T) {
	s := &Styler{color: true}
	if got := s.Spark(nil); got != "" {
		t.Errorf("Spark(nil) = %q, want empty", got)
	}
	if got := s.Spark([]float64{0, 0}); got != "▁▁" {
		t.Errorf("Spark(zeros) = %q, want %q", got, "▁▁")
	}
}
