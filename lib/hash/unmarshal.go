package hash

import (
	"errors"
)

func hexcharToByte(ch byte) (byte, error) {
	if ch >= '0' && ch <= '9' {
		return ch - '0', nil
	}
	if ch >= 'a' && ch <= 'f' {
		return ch - 'a' + 10, nil
	}
	return 0, errors.New("bad character")
}

func (h *Hash) unmarshalText(text []byte) error {
	for index, ch := range text {
		if index>>1 >= len(h) {
			return errors.New("hash string too long")
		}
		val, err := hexcharToByte(ch)
		if err != nil {
			return err
		}
		if index&1 == 0 {
			h[index>>1] += val << 4
		} else {
			h[index>>1] += val
		}
	}
	return nil
}
