//go:build !linux

package net

import (
	"errors"
)

func createTapDevice(params TapDeviceParams) (*TapDevice, error) {
	return nil, errors.New("tap devices not implemented on this OS")
}

func (td *TapDevice) close() error {
	return errors.New("tap devices not implemented on this OS")
}
