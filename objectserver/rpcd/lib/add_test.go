package lib

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/hash"
	"github.com/Cloud-Foundations/Dominator/lib/log"
	"github.com/Cloud-Foundations/Dominator/lib/log/testlogger"
	oclient "github.com/Cloud-Foundations/Dominator/lib/objectserver/client"
	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

var (
	object0 = []byte{0x01, 0x02, 0x03, 0x04}
	object1 = []byte{0x05, 0x06, 0x07}
	object2 = []byte{0x08, 0x09, 0x0a, 0x0b, 0x0c}
)

type objectsAdder interface {
	AddObjects(*srpc.Conn, srpc.Decoder, srpc.Encoder) error
	Ping(conn *srpc.Conn, request pingRequest, reply *pingResponse) error
}

type objectAdderType struct {
	failAfter  uint
	numObjects uint
}

type pingRequest struct {
	Data string
}

type pingResponse struct {
	Data string
}

type testReceiverType struct {
	logger      log.Logger
	objectAdder *objectAdderType
}

func makeObjectsAdderClientAndServer(rcvr objectsAdder) (*srpc.Client, error) {
	listener, err := net.Listen("tcp", "localhost:")
	if err != nil {
		return nil, err
	}
	go http.Serve(listener, nil)
	srpc.RegisterName("ObjectServer", rcvr)
	time.Sleep(time.Millisecond)
	stopTime := time.Now().Add(time.Second)
	for ; time.Until(stopTime) > 0; time.Sleep(time.Millisecond) {
		client, err := srpc.DialHTTP("tcp", listener.Addr().String(),
			100*time.Millisecond)
		if err != nil {
			return nil, err
		}
		if err := client.Ping(); err != nil {
			return nil, err
		}
		request := pingRequest{Data: "mydata"}
		var response pingResponse
		err = client.RequestReply("ObjectServer.Ping", request, &response)
		if err != nil {
			return nil, err
		}
		if response.Data != request.Data {
			return nil,
				fmt.Errorf("response.Data: \"%s\" != request.Data: \"%s\"",
					response.Data, request.Data)
		}
		return client, nil
	}
	return nil, errors.New("timed out connecting to server")
}

func sendObject(t *testing.T, oaQueue *oclient.ObjectAdderQueue,
	object []byte) error {
	time.Sleep(time.Millisecond)
	t.Logf("Sending object with length: %d", len(object))
	_, err := oaQueue.Add(bytes.NewReader(object), uint64(len(object)))
	return err
}

func TestQueue(t *testing.T) {
	srpcObj := &testReceiverType{
		logger: testlogger.New(t),
		objectAdder: &objectAdderType{
			failAfter: 4,
		},
	}
	srpcClient, err := makeObjectsAdderClientAndServer(srpcObj)
	if err != nil {
		t.Fatal(err)
	}
	oaQueue, err := oclient.NewObjectAdderQueue(srpcClient)
	if err != nil {
		t.Fatal(err)
	}
	if err := sendObject(t, oaQueue, object0); err != nil {
		t.Fatal(err)
	}
	if err := sendObject(t, oaQueue, object1); err != nil {
		t.Fatal(err)
	}
	if err := sendObject(t, oaQueue, object2); err != nil {
		t.Fatal(err)
	}
	if err := oaQueue.Close(); err != nil {
		t.Fatal(err)
	}
	if err := srpcClient.Ping(); err != nil {
		t.Fatalf("Error pinging: %s", err)
	}
	oaQueue, err = oclient.NewObjectAdderQueue(srpcClient)
	if err != nil {
		t.Fatal(err)
	}
	if err := sendObject(t, oaQueue, object0); err != nil {
		t.Fatal(err)
	}
	if err := sendObject(t, oaQueue, object1); err != nil {
		t.Fatal(err)
	}
	if err := sendObject(t, oaQueue, object2); err != nil {
		if err := oaQueue.Close(); err != nil {
			t.Fatal("extra error consumed")
		}
	} else if err := oaQueue.Close(); err == nil {
		t.Fatal("no error consumed")
	}
	if err := srpcClient.Ping(); err != nil {
		t.Fatalf("Error pinging: %s", err)
	}
}

func (oa *objectAdderType) AddObject(reader io.Reader, length uint64,
	expectedHash *hash.Hash) (hash.Hash, bool, error) {
	if _, err := io.CopyN(io.Discard, reader, int64(length)); err != nil {
		return hash.Hash{}, false, err
	}
	if oa.numObjects >= oa.failAfter {
		return hash.Hash{}, false, errors.New("add error")
	}
	oa.numObjects++
	return *expectedHash, true, nil
}

func (t *testReceiverType) AddObjects(conn *srpc.Conn, decoder srpc.Decoder,
	encoder srpc.Encoder) error {
	t.logger.Println("Calling AddObjects()")
	return AddObjects(conn, decoder, encoder, t.objectAdder, t.logger)
}

func (t *testReceiverType) Ping(conn *srpc.Conn,
	request pingRequest, reply *pingResponse) error {
	*reply = pingResponse{Data: request.Data}
	return nil
}
