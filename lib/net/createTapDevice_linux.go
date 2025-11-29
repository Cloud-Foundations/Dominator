package net

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
)

const (
	cIFF_TUN         = 0x0001
	cIFF_TAP         = 0x0002
	cIFF_NO_PI       = 0x1000
	cIFF_MULTI_QUEUE = 0x0100

	tunDevice = "/dev/net/tun"
)

type ifReq struct {
	Name  [0x10]byte
	Flags uint16
	pad   [0x28 - 0x10 - 2]byte
}

func createTapDevice(params TapDeviceParams) (*TapDevice, error) {
	file0, err := os.OpenFile(tunDevice, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	td := &TapDevice{
		Files: []*os.File{file0},
	}
	doClose := true
	defer func() {
		if doClose {
			td.Close()
		}
	}()
	req0 := ifReq{Flags: cIFF_TAP | cIFF_NO_PI}
	if params.NumQueues > 1 {
		req0.Flags |= cIFF_MULTI_QUEUE
	}
	err = wsyscall.Ioctl(int(file0.Fd()), syscall.TUNSETIFF,
		uintptr(unsafe.Pointer(&req0)))
	if err != nil {
		return nil, err
	}
	td.Name = strings.Trim(string(req0.Name[:]), "\x00")
	for index := uint(1); index < params.NumQueues; index++ {
		file, err := os.OpenFile("/dev/net/tun", os.O_RDWR, 0)
		if err != nil {
			return nil, err
		}
		td.Files = append(td.Files, file)
		newReq := ifReq{
			Flags: req0.Flags,
			Name:  req0.Name,
		}
		err = wsyscall.Ioctl(int(file.Fd()), syscall.TUNSETIFF,
			uintptr(unsafe.Pointer(&newReq)))
		if err != nil {
			return nil, err
		}
		newName := strings.Trim(string(newReq.Name[:]), "\x00")
		if newName != td.Name {
			return nil, fmt.Errorf("name[0]: \"%s\" != name[%d]: \"%s\"\n",
				td.Name, index, newName)
		}
	}
	doClose = false
	return td, nil
}

func (td *TapDevice) close() error {
	var err error
	for _, file := range td.Files {
		if e := file.Close(); e != nil && err == nil {
			err = e
		}
	}
	return err
}
