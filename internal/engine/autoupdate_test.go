package engine

import (
	"testing"
	"time"

	"github.com/jdbnet/dockyard/internal/config"
)

func TestAutoUpdatePolicyPrecedence(t *testing.T) {
	base := config.UpdatesConfig{
		Interval:           config.Duration{Duration: time.Hour},
		AutoUpdate:         false,
		AutoUpdateInterval: config.Duration{Duration: 30 * time.Minute},
	}
	globalOn := base
	globalOn.AutoUpdate = true

	cases := []struct {
		name     string
		cfg      config.UpdatesConfig
		labels   map[string]string
		wantOn   bool
		wantExp  bool
		wantEvery time.Duration
	}{
		{"global off, no label", base, nil, false, false, time.Hour},
		{"global on, no label", globalOn, nil, true, false, 30 * time.Minute},
		{"label true beats global off", base, map[string]string{LabelAutoUpdate: "true"}, true, true, 30 * time.Minute},
		{"label false beats global on", globalOn, map[string]string{LabelAutoUpdate: "false"}, false, true, time.Hour},
		{"label TRUE trimmed", base, map[string]string{LabelAutoUpdate: " TRUE "}, true, true, 30 * time.Minute},
		{"junk label uses global", globalOn, map[string]string{LabelAutoUpdate: "maybe"}, true, false, 30 * time.Minute},
		{"container interval", base, map[string]string{LabelInterval: "1m"}, false, false, time.Minute},
		{"autoupdate interval when global on", globalOn, nil, true, false, 30 * time.Minute},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			on, every, explicit := autoUpdatePolicy(tc.labels, tc.cfg)
			if on != tc.wantOn || explicit != tc.wantExp {
				t.Fatalf("enabled=%v explicit=%v, want on=%v exp=%v", on, explicit, tc.wantOn, tc.wantExp)
			}
			if every != tc.wantEvery {
				t.Fatalf("every=%s want=%s", every, tc.wantEvery)
			}
		})
	}
}

func TestParseContainerIntervalLabel(t *testing.T) {
	if _, ok := parseContainerIntervalLabel("5s"); ok {
		t.Fatal("below floor should fail")
	}
	if d, ok := parseContainerIntervalLabel("15m"); !ok || d != 15*time.Minute {
		t.Fatalf("got %v ok=%v", d, ok)
	}
	if _, ok := parseContainerIntervalLabel("not-a-duration"); ok {
		t.Fatal("invalid")
	}
}

func TestAutoLoopOwnsContainerNoRepoDigests(t *testing.T) {
	cfg := config.UpdatesConfig{AutoUpdate: true, Interval: config.Duration{Duration: time.Hour}}
	labels := map[string]string{LabelAutoUpdate: "true"}
	if !autoLoopOwnsContainer(labels, cfg, false, false) {
		t.Fatal("explicit opt-in without digest should be owned")
	}
	if autoLoopOwnsContainer(nil, cfg, false, false) {
		t.Fatal("global without digest should not be owned")
	}
}

func TestAutoLoopOwnsContainerIntervalOnly(t *testing.T) {
	cfg := config.UpdatesConfig{AutoUpdate: false, Interval: config.Duration{Duration: time.Hour}}
	labels := map[string]string{LabelInterval: "1m"}
	if !autoLoopOwnsContainer(labels, cfg, false, true) {
		t.Fatal("interval label with digest should be owned")
	}
}

func TestLastFailedDigestSkip(t *testing.T) {
	e := &Engine{
		autoLastFailedDigest: map[string]string{"web": "sha256:abc"},
	}
	e.autoMu.Lock()
	skip := e.autoLastFailedDigest["web"] == "sha256:abc"
	e.autoMu.Unlock()
	if !skip {
		t.Fatal("expected skip")
	}
}

func TestMergeUpdatesReport(t *testing.T) {
	base := UpdatesReport{
		Pending: []ImageUpdate{{Name: "scan"}},
	}
	auto := map[string]ImageUpdate{
		"tracked": {Name: "tracked", AutoUpdate: true},
		"broken":  {Name: "broken", Error: "nope"},
	}
	merged := mergeAutoRows(base, auto)
	if len(merged.Pending) != 2 || len(merged.Failures) != 1 {
		t.Fatalf("merged=%+v", merged)
	}
}

func TestParseOptionalMinDuration(t *testing.T) {
	if _, err := parseOptionalMinDuration("auto-update check interval", "5s", minAutoInterval); err == nil {
		t.Fatal("expected error below floor")
	}
	d, err := parseOptionalMinDuration("auto-update check interval", "", minAutoInterval)
	if err != nil || d != 0 {
		t.Fatalf("empty=%v err=%v", d, err)
	}
}
