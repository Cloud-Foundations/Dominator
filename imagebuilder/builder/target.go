package builder

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/Cloud-Foundations/Dominator/lib/goroutine"
	"github.com/Cloud-Foundations/Dominator/lib/wsyscall"
)

// newNamespaceTarget will create a goroutine which is locked to an OS
// thread with a separate mount namespace.
func newNamespaceTarget() (*goroutine.Goroutine, error) {
	g := goroutine.New()
	var err error
	g.Run(func() { err = wsyscall.UnshareMountNamespace() })
	if err != nil {
		return nil, err
	}
	return g, nil
}

// newNamespaceTargetWithMounts will create a goroutine which is locked to an OS
// thread with a separate mount namespace. The directories specified by
// bindMounts will be mounted into the new namespace.
func newNamespaceTargetWithMounts(rootDir string, bindMounts []string) (
	*goroutine.Goroutine, error) {
	g, err := newNamespaceTarget()
	if err != nil {
		return nil, err
	}
	g.Run(func() { err = setupMounts(rootDir, bindMounts) })
	if err != nil {
		return nil, err
	}
	return g, nil
}

// Run a command in the target root directory in the specified namespace and
// with a new PID namespace.
func runInTarget(g *goroutine.Goroutine, input io.Reader, output io.Writer,
	rootDir string, envGetter environmentGetter,
	prog string, args ...string) error {
	var environmentToInject map[string]string
	if envGetter != nil {
		environmentToInject = envGetter.getenv()
	}
	cmd := exec.Command(prog, args...)
	cmd.Env = stripVariables(os.Environ(), environmentToCopy, environmentToSet,
		environmentToInject)
	cmd.Dir = "/"
	cmd.Stdin = input
	cmd.Stdout = output
	cmd.Stderr = output
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Chroot:     rootDir,
		Setsid:     true,
		Cloneflags: syscall.CLONE_NEWPID,
	}
	var err error
	g.Run(func() { err = cmd.Start() })
	if err != nil {
		return err
	}
	return cmd.Wait()
}

// setupMounts will mutate the current namespace.
func setupMounts(rootDir string, bindMounts []string) error {
	err := wsyscall.Mount("none", filepath.Join(rootDir, "proc"), "proc", 0, "")
	if err != nil {
		return err
	}
	err = wsyscall.Mount("none", filepath.Join(rootDir, "sys"), "sysfs", 0, "")
	if err != nil {
		return err
	}
	for _, bindMount := range bindMounts {
		err := wsyscall.Mount(bindMount,
			filepath.Join(rootDir, bindMount), "",
			wsyscall.MS_BIND|wsyscall.MS_RDONLY, "")
		if err != nil {
			return fmt.Errorf("error bind mounting: %s: %s", bindMount, err)
		}
	}
	return nil
}

func stripVariables(input []string, varsToCopy map[string]struct{},
	varsToSet ...map[string]string) []string {
	output := make([]string, 0)
	for _, nameValue := range os.Environ() {
		split := strings.SplitN(nameValue, "=", 2)
		if len(split) == 2 {
			if _, ok := varsToCopy[split[0]]; ok {
				output = append(output, nameValue)
			}
		}
	}
	for _, varTable := range varsToSet {
		for name, value := range varTable {
			output = append(output, name+"="+value)
		}
	}
	sort.Strings(output)
	return output
}
