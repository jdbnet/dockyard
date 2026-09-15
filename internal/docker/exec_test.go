package docker

import "testing"

func TestIsMissingExecutable(t *testing.T) {
	if !isMissingExecutable(errString("OCI runtime exec failed: exec failed: unable to start container process: exec: \"/bin/bash\": executable file not found in $PATH")) {
		t.Fatal("expected missing executable")
	}
	if isMissingExecutable(errString("permission denied")) {
		t.Fatal("unexpected missing executable")
	}
}

func TestShellCommands(t *testing.T) {
	cmds := ShellCommands()
	if len(cmds) != 2 || cmds[0][0] != "/bin/bash" || cmds[1][0] != "/bin/sh" {
		t.Fatalf("unexpected shell commands: %+v", cmds)
	}
}

type errString string

func (e errString) Error() string { return string(e) }
