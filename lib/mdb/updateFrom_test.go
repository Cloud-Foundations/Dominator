package mdb

import (
	"testing"
)

func TestUpdateFromDifferentHostname(t *testing.T) {
	source := makeNonzeroMachine(t, -1)
	dest := &Machine{
		Hostname:             "some.host",
		DataSourceIdentifier: source.DataSourceIdentifier,
		DataSourceType:       source.DataSourceType,
	}
	dest.UpdateFrom(source)
	defaultMachine := &Machine{
		Hostname:             "some.host",
		DataSourceIdentifier: source.DataSourceIdentifier,
		DataSourceType:       source.DataSourceType,
	}
	if !dest.Compare(*defaultMachine) {
		t.Errorf("UpdateFrom(): copied data despite Hostname mismatch: %v",
			*dest)
	}
}

func TestUpdateFromSameHostname(t *testing.T) {
	source := makeNonzeroMachine(t, -1)
	dest := &Machine{
		Hostname:             source.Hostname,
		DataSourceIdentifier: source.DataSourceIdentifier,
		DataSourceType:       source.DataSourceType,
	}
	dest.UpdateFrom(source)
	if !dest.Compare(source) {
		t.Errorf("UpdateFrom(): %v != %v", *dest, source)
	}
}

func TestUpdateFromDifferentDataSourceIdentifier(t *testing.T) {
	source := makeNonzeroMachine(t, -1)
	dest := &Machine{
		Hostname:             source.Hostname,
		DataSourceIdentifier: "some.data.source.identifier",
		DataSourceType:       source.DataSourceType,
	}
	dest.UpdateFrom(source)
	if dest.DataSourceIdentifier != "" {
		t.Errorf("dest.DataSourceIdentifier: \"%s\" != empty string",
			dest.DataSourceIdentifier)
	}
}

func TestUpdateFromDifferentDataSourceType(t *testing.T) {
	source := makeNonzeroMachine(t, -1)
	dest := &Machine{
		Hostname:             source.Hostname,
		DataSourceIdentifier: source.DataSourceIdentifier,
		DataSourceType:       "some.data.source.type",
	}
	dest.UpdateFrom(source)
	if dest.DataSourceType != "" {
		t.Errorf("dest.DataSourceType: \"%s\" != empty string",
			dest.DataSourceType)
	}
}
