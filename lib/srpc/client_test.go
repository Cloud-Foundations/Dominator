package srpc

import (
	"os"
	"testing"
	"time"
)

func TestDialCloseClose(t *testing.T) {
	addr, err := makeListener(true, false)
	if err != nil {
		t.Fatal(err)
	}
	origNumOpenClients := numOpenClientConnections
	client, err := DialHTTP("tcp", addr.String(), 100*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if client.IsFromClientResource() {
		t.Fatal("IsFromClientResource() returned true, should be false")
	}
	if numOpenClientConnections != origNumOpenClients+1 {
		t.Fatalf("numOpenClientConnections: %d != %d",
			numOpenClientConnections, origNumOpenClients+1)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	if numOpenClientConnections != origNumOpenClients {
		t.Fatalf("numOpenClientConnections: %d != %d",
			numOpenClientConnections, origNumOpenClients)
	}
	if err := client.Close(); err != nil && err != os.ErrClosed {
		t.Fatal(err)
	}
	if numOpenClientConnections != origNumOpenClients {
		t.Fatalf("numOpenClientConnections: %d != %d",
			numOpenClientConnections, origNumOpenClients)
	}
}
