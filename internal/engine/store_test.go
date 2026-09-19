package engine

import "testing"

func TestApplyLiveStats(t *testing.T) {
	s := newStore()
	s.setContainers([]Container{
		{ID: "abc", Name: "web", State: "running"},
		{ID: "def", Name: "db", State: "running"},
	})
	spark := []StatPoint{{CPUPct: 12.5, MemPct: 40, MemBytes: 1024}}
	s.applyLiveStats("abc", spark[0], spark)

	got := s.containersSnapshot()
	if got[0].CPUPct != 12.5 || got[0].MemBytes != 1024 {
		t.Fatalf("web stats not applied: %+v", got[0])
	}
	if len(got[0].Sparkline) != 1 {
		t.Fatalf("expected sparkline, got %+v", got[0].Sparkline)
	}
	if got[1].CPUPct != 0 {
		t.Fatalf("db should be unchanged: %+v", got[1])
	}

	spark[0].CPUPct = 99
	if got[0].Sparkline[0].CPUPct != 12.5 {
		t.Fatal("sparkline slice should be copied")
	}
}
