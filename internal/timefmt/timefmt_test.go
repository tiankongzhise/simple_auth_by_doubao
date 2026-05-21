package timefmt

import "testing"

func TestBeijingLocal(t *testing.T) {
	got := BeijingLocal(1779415200)
	if got != "2026-05-22 10:00:00" {
		t.Fatalf("BeijingLocal() = %q", got)
	}
	if got := BeijingLocal(0); got != "" {
		t.Fatalf("BeijingLocal(0) = %q, want empty", got)
	}
}
