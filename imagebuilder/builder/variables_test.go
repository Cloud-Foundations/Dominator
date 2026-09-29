package builder

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Cloud-Foundations/Dominator/lib/expand"
	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
)

var (
	testBuilder     = &Builder{}
	testMappingFunc = func(name string) string {
		return testStream.getenv()[name]
	}
	testStream = &imageStreamType{
		name: "users/fred/generic/base/Debian-10/amd64",
	}
)

func TestVariablesGetterAdder(t *testing.T) {
	imageStream := &imageStreamType{}
	vGetter := variablesGetter(imageStream.getenv()).copy()
	key := "key"
	value := "value"
	vGetter.add(key, value)
	result := vGetter.getenv()[key]
	if result != value {
		t.Errorf("expected: %s got: %s", value, result)
	}
}

func TestSimpleExpressionExpansion(t *testing.T) {
	result := expand.Expression("${IMAGE_STREAM}", testMappingFunc)
	if result != testStream.name {
		t.Errorf("expected: %s got: %s", testStream.name, result)
	}
	result = expand.Expression("${IMAGE_STREAM_DIRECTORY_NAME}",
		testMappingFunc)
	expected := "users/fred/generic/base/Debian-10"
	if result != expected {
		t.Errorf("expected: %s got: %s", expected, result)
	}
	result = expand.Expression("${IMAGE_STREAM_LEAF_NAME}", testMappingFunc)
	expected = "amd64"
	if result != expected {
		t.Errorf("expected: %s got: %s", expected, result)
	}
}

func TestMergeManifestVariables(t *testing.T) {
	vGetter := variablesGetter(testStream.getenv()).copy()
	vGetter.add("RELEASE", "testing")
	vGetter.mergeManifest(map[string]string{
		"RELEASE": "stable",
		"LEAF":    "${IMAGE_STREAM_LEAF_NAME}",
		"UNKNOWN": "${NOT_SET}",
	})
	if result := vGetter.getenv()["RELEASE"]; result != "stable" {
		t.Errorf("expected: %s got: %s", "stable", result)
	}
	if result := vGetter.getenv()["LEAF"]; result != "amd64" {
		t.Errorf("expected: %s got: %s", "amd64", result)
	}
	if _, ok := vGetter.getenv()["UNKNOWN"]; ok {
		t.Errorf("expected UNKNOWN to be absent")
	}
	vGetter.merge(variablesGetter{"RELEASE": "unstable"})
	if result := vGetter.getenv()["RELEASE"]; result != "unstable" {
		t.Errorf("expected: %s got: %s", "unstable", result)
	}
}

func TestManifestVariablesExpansion(t *testing.T) {
	manifestDir := t.TempDir()
	manifest := `{
	    "SourceImage": "base/Debian-10/$IMAGE_STREAM_LEAF_NAME",
	    "SourceImageBuildVariables": {"RELEASE": "$RELEASE"},
	    "SourceImageTagsToMatch": {"Release": ["$RELEASE"]},
	    "Variables": {"RELEASE": "stable"}
	}`
	err := os.WriteFile(filepath.Join(manifestDir, "manifest"),
		[]byte(manifest), fsutil.PublicFilePerms)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name             string
		requestVariables variablesGetter
		expected         string
	}{
		{"manifest variable", nil, "stable"},
		{"build request override",
			variablesGetter{"RELEASE": "unstable"}, "unstable"},
	} {
		rawManifestConfig, err := readManifestFile(manifestDir, nil)
		if err != nil {
			t.Fatal(err)
		}
		vGetter := variablesGetter(testStream.getenv()).copy()
		vGetter.mergeManifest(rawManifestConfig.Variables)
		vGetter.merge(tc.requestVariables)
		manifestConfig, err := readManifestFile(manifestDir, vGetter)
		if err != nil {
			t.Fatal(err)
		}
		result := manifestConfig.SourceImageBuildVariables["RELEASE"]
		if result != tc.expected {
			t.Errorf("%s: SourceImageBuildVariables expected: %s got: %s",
				tc.name, tc.expected, result)
		}
		matchValues := manifestConfig.SourceImageTagsToMatch["Release"]
		if len(matchValues) != 1 || matchValues[0] != tc.expected {
			t.Errorf("%s: SourceImageTagsToMatch expected: [%s] got: %v",
				tc.name, tc.expected, matchValues)
		}
	}
}

func TestManifestWithoutVariables(t *testing.T) {
	manifestDir := t.TempDir()
	manifest := `{
	    "SourceImage": "base/Debian-10/$IMAGE_STREAM_LEAF_NAME",
	    "SourceImageTagsToMatch": {"Release": ["$RELEASE"]}
	}`
	err := os.WriteFile(filepath.Join(manifestDir, "manifest"),
		[]byte(manifest), fsutil.PublicFilePerms)
	if err != nil {
		t.Fatal(err)
	}
	rawManifestConfig, err := readManifestFile(manifestDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	vGetter := variablesGetter(testStream.getenv()).copy()
	vGetter.mergeManifest(rawManifestConfig.Variables)
	manifestConfig, err := readManifestFile(manifestDir, vGetter)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := manifestConfig.SourceImageTagsToMatch["Release"]; ok {
		t.Errorf("expected the Release tag match to be deleted")
	}
	expected := "base/Debian-10/amd64"
	if manifestConfig.SourceImage != expected {
		t.Errorf("expected: %s got: %s", expected, manifestConfig.SourceImage)
	}
}
