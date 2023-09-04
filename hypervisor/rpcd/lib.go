package rpcd

import (
	"io"

	"github.com/Cloud-Foundations/Dominator/lib/srpc"
)

func connectChannelsToConnection(conn *srpc.Conn, input chan<- byte,
	output <-chan byte) error {
	closeNotifier := make(chan error, 1)
	go func() { // Read from connection and write to input until EOF.
		buffer := make([]byte, 256)
		for {
			if nRead, err := conn.Read(buffer); err != nil {
				if err != io.EOF {
					closeNotifier <- err
				} else {
					closeNotifier <- srpc.ErrorCloseClient
				}
				return
			} else {
				for _, char := range buffer[:nRead] {
					input <- char
				}
			}
		}
	}()
	// Read from output until closure or transmission error.
	for {
		select {
		case data, ok := <-output:
			var buffer []byte
			if !ok {
				buffer = []byte("VM serial port closed\n")
			} else {
				buffer = readData(data, output)
			}
			if _, err := conn.Write(buffer); err != nil {
				return err
			}
			if err := conn.Flush(); err != nil {
				return err
			}
			if !ok {
				return srpc.ErrorCloseClient
			}
		case err := <-closeNotifier:
			return err
		}
	}
}

func readData(firstByte byte, moreBytes <-chan byte) []byte {
	buffer := make([]byte, 1, len(moreBytes)+1)
	buffer[0] = firstByte
	for {
		select {
		case char, ok := <-moreBytes:
			if !ok {
				return buffer
			}
			buffer = append(buffer, char)
		default:
			return buffer
		}
	}
}
