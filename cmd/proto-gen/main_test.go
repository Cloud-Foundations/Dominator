package main

import "testing"

func TestCheckOutFlags(t *testing.T) {
	tests := []struct {
		name                         string
		out, goPackage, pkg, service string
		wantErr                      bool
	}{
		{"scan only", "", "", "", "", false},
		{"generate", "f.proto", "g", "p", "S", false},
		{"generation flags without out", "", "g", "", "", true},
		{"package without out", "", "", "p", "", true},
		{"service without out", "", "", "", "S", true},
		{"out without anything", "f.proto", "", "", "", true},
		{"out without Go package", "f.proto", "", "p", "S", true},
		{"out without package", "f.proto", "g", "", "S", true},
		{"out without service", "f.proto", "g", "p", "", true},
	}
	for _, test := range tests {
		err := checkOutFlags(test.out, test.goPackage, test.pkg, test.service)
		if (err != nil) != test.wantErr {
			t.Errorf("%s: got error %v, want error %v", test.name, err,
				test.wantErr)
		}
	}
}
