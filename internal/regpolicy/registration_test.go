package regpolicy

import "testing"

func TestRegistrationShouldDisableAdRewards(t *testing.T) {
	tests := []struct {
		name     string
		devCnt   int
		ipCnt    int
		devLim   int
		ipLim    int
		hasDev   bool
		evalIP   bool
		wantDis  bool
	}{
		{"no limits", 10, 10, 0, 0, true, true, false},
		{"device skip empty id", 5, 0, 2, 5, false, false, false},
		{"device first ok lim2", 0, 0, 2, 5, true, false, false},
		{"device second ok lim2", 1, 0, 2, 5, true, false, false},
		{"device third blocked lim2", 2, 0, 2, 5, true, false, true},
		{"ip first ok lim3", 0, 0, 0, 3, false, true, false},
		{"ip blocked at 3", 0, 3, 0, 3, false, true, true},
		{"either fires", 0, 3, 2, 3, false, true, true},
		{"either fires device", 2, 0, 2, 5, true, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RegistrationShouldDisableAdRewards(tt.devCnt, tt.ipCnt, tt.devLim, tt.ipLim, tt.hasDev, tt.evalIP)
			if got != tt.wantDis {
				t.Fatalf("got %v want %v", got, tt.wantDis)
			}
		})
	}
}
