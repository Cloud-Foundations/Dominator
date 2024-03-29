package srpc

import (
	"os"
	"testing"
)

func TestGetCallCloseCall(t *testing.T) {
	addr, err := makeListener(true, false)
	if err != nil {
		t.Fatal(err)
	}
	cr := NewClientResource("tcp", addr.String())
	client, err := cr.GetHTTP(nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := testDoCallPlain(t, client, "get+plain"); err != nil {
		t.Fatal(err)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := recover(); err == nil {
			t.Fatal("call on closed client did not panic")
		}
	}()
	if err := testDoCallPlain(t, client, "get+close+plain1"); err == nil {
		t.Fatal("call on close client did not fail")
	}
}

func TestGetCallPutCall(t *testing.T) {
	addr, err := makeListener(true, false)
	if err != nil {
		t.Fatal(err)
	}
	cr := NewClientResource("tcp", addr.String())
	client, err := cr.GetHTTP(nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := testDoCallPlain(t, client, "get+plain"); err != nil {
		t.Fatal(err)
	}
	client.Put()
	defer func() {
		if err := recover(); err == nil {
			t.Fatal("call on put client did not panic")
		}
	}()
	if err := testDoCallPlain(t, client, "get+put+plain0"); err != nil {
		t.Fatal(err)
	}
}

func TestGetCloseClose(t *testing.T) {
	addr, err := makeListener(true, false)
	if err != nil {
		t.Fatal(err)
	}
	origNumOpenClients := numOpenClientConnections
	cr := NewClientResource("tcp", addr.String())
	client, err := cr.GetHTTP(nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !client.IsFromClientResource() {
		t.Fatal("IsFromClientResource() returned false, should be true")
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
