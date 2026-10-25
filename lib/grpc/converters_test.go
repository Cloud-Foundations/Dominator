package grpc

import (
	"net"
	"reflect"
	"testing"

	"github.com/Cloud-Foundations/Dominator/lib/errors"
	"github.com/Cloud-Foundations/Dominator/lib/tags"
)

func TestIPConversion(t *testing.T) {
	for _, test := range []struct{ in, want string }{
		{"", ""},
		{"10.1.2.3", "10.1.2.3"},
		{"2001:db8::1", "2001:db8::1"},
		{"::ffff:10.1.2.3", "10.1.2.3"},
	} {
		ip, err := StringToIP(test.in)
		if err != nil {
			t.Fatalf("StringToIP(%q): %s", test.in, err)
		}
		if test.in == "" && ip != nil {
			t.Errorf("StringToIP(%q) = %v, want nil", test.in, ip)
		}
		if got := IPToString(ip); got != test.want {
			t.Errorf("IPToString(%q) = %q, want %q", test.in, got, test.want)
		}
	}
	if got := IPToString(net.IP{}); got != "" {
		t.Errorf("IPToString(empty) = %q, want empty", got)
	}
}

func TestStringToIPMalformed(t *testing.T) {
	_, err := StringToIP("10.1.2")
	if _, ok := err.(*errors.InvalidArgumentError); !ok {
		t.Errorf("StringToIP(malformed) error = %v, want invalid argument", err)
	}
}

func TestMACRoundTrip(t *testing.T) {
	for _, s := range []string{"", "52:54:00:12:34:56"} {
		mac, err := StringToMAC(s)
		if err != nil {
			t.Fatalf("StringToMAC(%q): %s", s, err)
		}
		if got := MACToString(mac); got != s {
			t.Errorf("MACToString(%q) = %q", s, got)
		}
	}
	_, err := StringToMAC("not-a-mac")
	if _, ok := err.(*errors.InvalidArgumentError); !ok {
		t.Errorf("StringToMAC(malformed) error = %v, want invalid argument", err)
	}
}

func TestTagsConversion(t *testing.T) {
	if TagsToMap(nil) != nil || MapToTags(nil) != nil {
		t.Error("nil should convert to nil")
	}
	original := tags.Tags{"a": "1", "b": "2"}
	m := TagsToMap(original)
	if !reflect.DeepEqual(m, map[string]string(original)) {
		t.Errorf("TagsToMap = %v", m)
	}
	m["a"] = "changed"
	if original["a"] != "1" {
		t.Error("TagsToMap aliased its input")
	}
	back := MapToTags(m)
	back["b"] = "changed"
	if m["b"] != "2" {
		t.Error("MapToTags aliased its input")
	}
}
