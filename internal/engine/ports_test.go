package engine

import "testing"

func TestParsePortString(t *testing.T) {
	host, container, proto, err := parsePortString("8080:80/tcp")
	if err != nil {
		t.Fatal(err)
	}
	if host != 8080 || container != 80 || proto != "tcp" {
		t.Fatalf("got %d:%d/%s", host, container, proto)
	}
}

func TestParsePortString_invalid(t *testing.T) {
	if _, _, _, err := parsePortString("bad"); err == nil {
		t.Fatal("expected error")
	}
}

func TestEnginePorts(t *testing.T) {
	e := &Engine{store: newStore()}
	e.store.setContainers([]Container{
		{
			ID:             "abc",
			Name:           "web",
			State:          "running",
			ComposeProject: "myapp",
			ComposeService: "web",
			Ports:          []string{"8080:80/tcp", "8443:443/tcp"},
		},
		{
			ID:    "def",
			Name:  "db",
			State: "running",
			Ports: []string{"5432:5432/tcp"},
		},
	})
	ports, err := e.Ports(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(ports) != 3 {
		t.Fatalf("expected 3 bindings, got %d", len(ports))
	}
	if ports[0].HostPort != 5432 || ports[0].ContainerName != "db" {
		t.Fatalf("expected sorted by host port: %+v", ports[0])
	}
	if ports[2].ComposeProject != "myapp" {
		t.Fatalf("expected compose project on last port: %+v", ports[2])
	}
}
