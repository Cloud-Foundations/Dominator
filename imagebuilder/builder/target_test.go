//go:build linux

package builder

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Cloud-Foundations/Dominator/lib/fsutil"
)

// TestBuildCommandProcMatchesPidNamespace checks that a build command sees a
// /proc for its own PID namespace. Package scripts find a daemon in /proc and
// kill it by pid; if /proc shows the host's pids, the kill hits a process of
// the build instead.
func TestBuildCommandProcMatchesPidNamespace(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("needs root")
	}
	rootDir := t.TempDir()
	var bindMounts []bindMountType
	// Use the host's shell and tools in the build root.
	for _, dir := range []string{"bin", "lib", "lib64", "proc", "sbin", "sys",
		"usr"} {
		hostDir := filepath.Join("/", dir)
		rootPath := filepath.Join(rootDir, dir)
		if link, err := os.Readlink(hostDir); err == nil {
			if err := os.Symlink(link, rootPath); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.Mkdir(rootPath, 0755); err != nil {
			t.Fatal(err)
		}
		if dir != "proc" && dir != "sys" {
			bindMounts = append(bindMounts,
				bindMountType{source: hostDir, target: "/" + dir})
		}
	}
	// A host process for the build to find, as polkitd was on the imaginator.
	daemonPath := filepath.Join(t.TempDir(), "fake-polkitd")
	if err := fsutil.CopyFile(daemonPath, "/bin/sleep", 0755); err != nil {
		t.Fatal(err)
	}
	daemon := exec.Command(daemonPath, "60")
	if err := daemon.Start(); err != nil {
		t.Fatal(err)
	}
	defer daemon.Wait()
	defer daemon.Process.Kill()
	g, err := newNamespaceTargetWithMounts(rootDir, bindMounts)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Quit()
	// The shell opens /proc/self itself, so the pid read there is its own.
	// A correct /proc gives "1 1 false": the shell is pid 1, /proc agrees and
	// this test process (on the host) is not visible. Then start-stop-daemon
	// scans /proc as the polkitd postinst does; --test sends no signal.
	script := fmt.Sprintf(`read pid rest < /proc/self/stat
if [ -e /proc/%d ]; then host=true; else host=false; fi
echo $$ $pid $host
if [ -x /sbin/start-stop-daemon ]; then
	start-stop-daemon --stop --test --name fake-polkitd || true
fi`, os.Getpid())
	output := &bytes.Buffer{}
	err = runInTarget(context.Background(), g, nil, output, output, rootDir,
		nil, "/bin/sh", "-c", script)
	if err != nil {
		t.Fatalf("%s: %s", err, output)
	}
	lines := strings.SplitN(output.String(), "\n", 2)
	if lines[0] != "1 1 false" {
		t.Errorf("getpid, /proc/self and host pid visible: got %q, want %q",
			lines[0], "1 1 false")
	}
	if strings.Contains(output.String(), "Would send signal") {
		t.Errorf("start-stop-daemon in the build found host pid %d: %s",
			daemon.Process.Pid, output)
	}
}
