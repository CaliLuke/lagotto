package pkgload

import (
	"strings"
	"testing"
)

func TestCheckToolchainRejectsOlderBinary(t *testing.T) {
	root := writeModule(t, map[string]string{
		"go.mod": "module example.com/new\ngo 1.26\n",
	})
	err := checkToolchain(root, "go1.23.12")
	if err == nil {
		t.Fatal("expected an older Lagotto toolchain to be rejected")
	}
	for _, want := range []string{"requires Go 1.26", "built with go1.23.12", "newer Lagotto release"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}

func TestCheckToolchainAcceptsEqualOrNewerBinary(t *testing.T) {
	root := writeModule(t, map[string]string{
		"go.mod": "module example.com/compatible\ngo 1.26\n",
	})
	for _, builtWith := range []string{"go1.26.0", "go1.26.4", "go1.27rc1"} {
		if err := checkToolchain(root, builtWith); err != nil {
			t.Errorf("checkToolchain(%q): %v", builtWith, err)
		}
	}
}

func TestCheckToolchainHandlesRuntimeExperimentSuffix(t *testing.T) {
	root := writeModule(t, map[string]string{
		"go.mod": "module example.com/new\ngo 1.26\n",
	})
	if err := checkToolchain(root, "go1.25.7 X:boringcrypto"); err == nil {
		t.Fatal("expected suffixed older runtime version to be rejected")
	}
}

func TestCheckToolchainFindsModuleAboveSubdirectory(t *testing.T) {
	root := writeModule(t, map[string]string{
		"go.mod":      "module example.com/new\ngo 1.26\n",
		"pkg/file.go": "package pkg\n",
	})
	err := checkToolchain(root+"/pkg", "go1.25.7")
	if err == nil {
		t.Fatal("expected parent module requirement to be enforced")
	}
}

func TestCheckToolchainAllowsDirectoriesWithoutModule(t *testing.T) {
	if err := checkToolchain(t.TempDir(), "go1.23.0"); err != nil {
		t.Fatal(err)
	}
}
