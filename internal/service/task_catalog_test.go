package service

import "testing"

func TestBuildFullTaskCatalog_Counts(t *testing.T) {
	cat := BuildFullTaskCatalog()
	var p0, p1, p2 int
	for _, d := range cat {
		switch d.Tier {
		case TaskTierP0:
			p0++
		case TaskTierP1:
			p1++
		case TaskTierP2:
			p2++
		}
	}
	if p0 != 10 {
		t.Fatalf("p0=%d want 10", p0)
	}
	if p1 < 30 {
		t.Fatalf("p1=%d want >=30", p1)
	}
	if p2 < 5 {
		t.Fatalf("p2=%d want >=5", p2)
	}
}

func TestTaskTierPolicy_FiltersList(t *testing.T) {
	ReloadTaskRegistry("")
	SetTaskTierPolicy(TaskTierPolicy{P0Enabled: true, P1Enabled: false, P2Enabled: false})
	n0 := len(ListActiveTaskDefs())
	SetTaskTierPolicy(TaskTierPolicy{P0Enabled: true, P1Enabled: true, P2Enabled: false})
	n1 := len(ListActiveTaskDefs())
	if n1 <= n0 {
		t.Fatalf("p1 on active=%d want > p0-only %d", n1, n0)
	}
}
