package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/Cloud-Foundations/Dominator/lib/constants"
)

func checkExternallyPatchable() (bool, error) {
	resp, err := http.Get(constants.MetadataUrl +
		constants.MetadataExternallyPatchable)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return false, nil
	}
	if body, err := ioutil.ReadAll(resp.Body); err != nil {
		return false, err
	} else {
		value := strings.TrimSpace(string(body))
		if value == "true" {
			return true, nil
		} else {
			return false, nil
		}
	}
}

func runTestAndExit(test func() (bool, error)) {
	if patchable, err := checkExternallyPatchable(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	} else if patchable {
		os.Exit(0)
	}
	os.Exit(1)
}
