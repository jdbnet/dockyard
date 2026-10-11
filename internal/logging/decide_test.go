package logging

import "testing"

func TestWantLogs(t *testing.T) {
	cases := []struct {
		name    string
		labels  map[string]string
		def     bool
		collect bool
	}{
		{name: "default on", def: true, collect: true},
		{name: "default off", def: false, collect: false},
		{name: "opt out", labels: map[string]string{"dockyard.logs": "false"}, def: true, collect: false},
		{name: "opt in", labels: map[string]string{"dockyard.logs": "TRUE"}, def: false, collect: true},
		{name: "unknown label uses default", labels: map[string]string{"dockyard.logs": "maybe"}, def: true, collect: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			target := Target{ID: "container-id-0001", State: "running", LogDriver: "json-file", Labels: tc.labels}
			got := Decide(target, tc.def, nil)
			if got.Collect != tc.collect || got.NotCollectable != "" {
				t.Fatalf("Decide = %+v", got)
			}
		})
	}
}

func TestStreamLabels(t *testing.T) {
	labels := StreamLabels("host-a", Target{
		Name:           "api",
		Image:          "api:1",
		ComposeProject: "app",
		ComposeService: "api",
		Labels: map[string]string{
			"dockyard.logs.label.env-name": "prod",
			"dockyard.logs.label.host":     "nope",
			"dockyard.logs.label.1bad":     "x",
			"other":                        "skip",
		},
	}, "stderr")
	if labels["env_name"] != "prod" || labels["host"] != "host-a" || labels["container"] != "api" || labels["stream"] != "stderr" {
		t.Fatalf("labels = %#v", labels)
	}
	if labels["compose_project"] != "app" || labels["compose_service"] != "api" || labels["image"] != "api:1" {
		t.Fatalf("labels = %#v", labels)
	}
	if labels["_1bad"] != "x" {
		t.Fatalf("sanitised label = %#v", labels)
	}
	if _, ok := labels["other"]; ok {
		t.Fatal("unrelated docker label was copied")
	}
}

func TestSkipsSelfUnlessOptedIn(t *testing.T) {
	id := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	target := Target{ID: id, Name: "dockyard", State: "running", LogDriver: "local"}
	if got := Decide(target, true, []string{id[:12]}); got.Collect || got.NotCollectable != "" {
		t.Fatalf("self container should be skipped, got %+v", got)
	}
	target.Labels = map[string]string{"dockyard.logs": "true"}
	if got := Decide(target, false, []string{id}); !got.Collect {
		t.Fatalf("explicit opt-in should collect self, got %+v", got)
	}
}

func TestUnsupportedDriverIsNotCollectable(t *testing.T) {
	for _, driver := range []string{"syslog", "none", "journald", ""} {
		got := Decide(Target{ID: "container-id-0001", State: "running", LogDriver: driver}, true, nil)
		if got.Collect || got.NotCollectable == "" {
			t.Fatalf("driver %q: %+v", driver, got)
		}
	}
	if got := Decide(Target{ID: "container-id-0001", State: "running", LogDriver: "json-file"}, true, nil); !got.Collect {
		t.Fatalf("json-file should collect, got %+v", got)
	}
}
