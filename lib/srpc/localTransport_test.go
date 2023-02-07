package srpc

import (
	"runtime"
	"testing"
)

func TestUnixUpgrade(t *testing.T) {
	if runtime.GOOS != "linux" {
		return
	}
	oldAttemptTransportUpgrade := attemptTransportUpgrade
	attemptTransportUpgrade = true
	client, err := makeListenerAndConnect(true, false)
	if err != nil {
		t.Fatal(err)
	}
	if network := client.conn.RemoteAddr().Network(); network != "unix" {
		t.Fatalf("Expected unix connection, have: %s", network)
	}
	if err := testDoCallPlain(t, client, "Unix/plain"); err != nil {
		t.Fatal(err)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	attemptTransportUpgrade = oldAttemptTransportUpgrade
}
