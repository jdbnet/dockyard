package notify

import (
	"testing"
)

func TestPendingFingerprintOrderIndependent(t *testing.T) {
	a := []PendingRow{
		{ContainerID: "a", RemoteDigest: "sha256:1"},
		{ContainerID: "b", RemoteDigest: "sha256:2"},
	}
	b := []PendingRow{
		{ContainerID: "b", RemoteDigest: "sha256:2"},
		{ContainerID: "a", RemoteDigest: "sha256:1"},
	}
	if PendingFingerprint(a) != PendingFingerprint(b) {
		t.Fatal("expected same fingerprint")
	}
}

func TestPendingFingerprintChangesWithDigest(t *testing.T) {
	one := []PendingRow{{ContainerID: "a", RemoteDigest: "sha256:1"}}
	two := []PendingRow{{ContainerID: "a", RemoteDigest: "sha256:2"}}
	if PendingFingerprint(one) == PendingFingerprint(two) {
		t.Fatal("expected different fingerprint")
	}
}
