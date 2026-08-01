package paths

import "testing"

func TestResolve(t *testing.T) {
	l := Resolve("/tmp/herminas")
	if l.DataDir != "/tmp/herminas/data" {
		t.Fatalf("unexpected data dir: %s", l.DataDir)
	}
	if l.ConfigDir != "/tmp/herminas/config" {
		t.Fatalf("unexpected config dir: %s", l.ConfigDir)
	}
}
