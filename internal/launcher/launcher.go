// Package launcher hands control of the terminal to a coding-agent binary
// via syscall.Exec. The current process is replaced — there is no return.
//
// Bubble Tea must already be torn down before calling Exec.
package launcher

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// Exec changes directory to cwd and replaces this process with bin+args.
// args[0] is conventionally the program name (e.g. "claude").
func Exec(cwd, bin string, args []string) error {
	abs, err := exec.LookPath(bin)
	if err != nil {
		return fmt.Errorf("%s not found on PATH: %w", bin, err)
	}
	if err := syscall.Chdir(cwd); err != nil {
		return fmt.Errorf("chdir %s: %w", cwd, err)
	}
	return syscall.Exec(abs, args, os.Environ())
}
