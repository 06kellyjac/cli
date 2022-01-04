package compute

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	fsterr "github.com/fastly/cli/pkg/errors"
	fstexec "github.com/fastly/cli/pkg/exec"
	"github.com/fastly/cli/pkg/filesystem"
	"github.com/fastly/cli/pkg/text"
)

// JSSourceDirectory represents the source code directory.
const JSSourceDirectory = "src"

// JsToolchain represents the default JS toolchain.
const JsToolchain = "npm"

var errFormat = "To fix this error, run the following command:\n\n\t$ %s"

// JavaScript implements a Toolchain for the JavaScript language.
type JavaScript struct {
	Shell

	build               string
	errlog              fsterr.LogInterface
	packageDependency   string
	packageExecutable   string
	timeout             int
	toolchain           string
	validateScriptBuild bool
}

// NewJavaScript constructs a new JavaScript.
func NewJavaScript(timeout int, build string, errlog fsterr.LogInterface) *JavaScript {
	return &JavaScript{
		Shell:               Shell{},
		build:               build,
		errlog:              errlog,
		packageDependency:   "@fastly/js-compute",
		packageExecutable:   "js-compute-runtime",
		timeout:             timeout,
		toolchain:           JsToolchain,
		validateScriptBuild: true,
	}
}

// Initialize implements the Toolchain interface and initializes a newly cloned
// package by installing required dependencies.
func (j JavaScript) Initialize(out io.Writer) error {
	// 1) Check a.toolchain is on $PATH
	//
	// npm, a Node/JavaScript toolchain installer/manager, is needed to install
	// the package dependencies on initialization. We only check whether the
	// binary exists on the users $PATH and error with installation help text.
	fmt.Fprintf(out, "Checking if %s is installed...\n", j.toolchain)

	p, err := exec.LookPath(j.toolchain)
	if err != nil {
		j.errlog.Add(err)
		nodejsURL := "https://nodejs.org/"
		remediation := fmt.Sprintf("To fix this error, install Node.js and %s by visiting:\n\n\t$ %s", j.toolchain, text.Bold(nodejsURL))

		return fsterr.RemediationError{
			Inner:       fmt.Errorf("`%s` not found in $PATH", j.toolchain),
			Remediation: remediation,
		}
	}

	fmt.Fprintf(out, "Found %s at %s\n", j.toolchain, p)

	// 2) Check package.json file exists in $PWD
	//
	// A valid npm package manifest file is needed for the install command to
	// work. Therefore, we first assert whether one exists in the current $PWD.
	pkg, err := filepath.Abs("package.json")
	if err != nil {
		j.errlog.Add(err)
		return fmt.Errorf("getting package.json path: %w", err)
	}

	if !filesystem.FileExists(pkg) {
		remediation := "npm init"
		err := fsterr.RemediationError{
			Inner:       fmt.Errorf("package.json not found"),
			Remediation: fmt.Sprintf(errFormat, text.Bold(remediation)),
		}
		j.errlog.Add(err)
		return err
	}

	fmt.Fprintf(out, "Found package.json at %s\n", pkg)
	fmt.Fprintf(out, "Installing package dependencies...\n")

	cmd := fstexec.Streaming{
		Command: j.toolchain,
		Args:    []string{"install"},
		Env:     []string{},
		Output:  out,
	}
	return cmd.Exec()
}

// Verify implements the Toolchain interface and verifies whether the
// JavaScript language toolchain is correctly configured on the host.
func (j JavaScript) Verify(out io.Writer) error {
	// 1) Check a.toolchain is on $PATH
	//
	// npm, a popular Node/JavaScript toolchain installer/manager, which is needed
	// to assert that the correct versions of the js-compute-runtime compiler and
	// @fastly/js-compute package are installed.  We only check whether the binary
	// exists on the users $PATH and error with installation help text.
	fmt.Fprintf(out, "Checking if %s is installed...\n", j.toolchain)

	p, err := exec.LookPath(j.toolchain)
	if err != nil {
		j.errlog.Add(err)
		nodejsURL := "https://nodejs.org/"
		remediation := fmt.Sprintf("To fix this error, install Node.js and %s by visiting:\n\n\t$ %s", j.toolchain, text.Bold(nodejsURL))

		return fsterr.RemediationError{
			Inner:       fmt.Errorf("`%s` not found in $PATH", j.toolchain),
			Remediation: remediation,
		}
	}

	fmt.Fprintf(out, "Found %s at %s\n", j.toolchain, p)

	// 2) Check package.json file exists in $PWD
	//
	// A valid package is needed for compilation and to assert whether the
	// required dependencies are installed locally. Therefore, we first assert
	// whether one exists in the current $PWD.
	pkg, err := filepath.Abs("package.json")
	if err != nil {
		j.errlog.Add(err)
		return fmt.Errorf("getting package.json path: %w", err)
	}

	if !filesystem.FileExists(pkg) {
		remediation := "npm init"
		err := fsterr.RemediationError{
			Inner:       fmt.Errorf("package.json not found"),
			Remediation: fmt.Sprintf(errFormat, text.Bold(remediation)),
		}
		j.errlog.Add(err)
		return err
	}

	fmt.Fprintf(out, "Found package.json at %s\n", pkg)

	// 3) Check if `js-compute-runtime` is installed.
	//
	// js-compute-runtime is the JavaScript compiler. We first check if the
	// required dependency exists in the package.json and then whether the
	// js-compute-runtime binary exists in the toolchain bin directory.
	fmt.Fprintf(out, "Checking if %s is installed...\n", j.packageDependency)
	if !checkJsPackageDependencyExists(j.toolchain, j.packageDependency) {
		remediation := fmt.Sprintf("npm install --save-dev %s", j.packageDependency)
		err := fsterr.RemediationError{
			Inner:       fmt.Errorf("`%s` not found in package.json", j.packageDependency),
			Remediation: fmt.Sprintf(errFormat, text.Bold(remediation)),
		}
		j.errlog.Add(err)
		return err
	}

	p, err = getJsToolchainBinPath(j.toolchain)
	if err != nil {
		j.errlog.Add(err)
		remediation := "npm install --global npm@latest"
		return fsterr.RemediationError{
			Inner:       fmt.Errorf("could not determine %s bin path", j.toolchain),
			Remediation: fmt.Sprintf(errFormat, text.Bold(remediation)),
		}
	}

	path, err := exec.LookPath(filepath.Join(p, j.packageExecutable))
	if err != nil {
		j.errlog.Add(err)
		return fmt.Errorf("getting %s path: %w", j.packageExecutable, err)
	}
	if !filesystem.FileExists(path) {
		remediation := fmt.Sprintf("npm install --save-dev %s", j.packageDependency)
		err := fsterr.RemediationError{
			Inner:       fmt.Errorf("`%s` binary not found in %s", j.packageExecutable, p),
			Remediation: fmt.Sprintf(errFormat, text.Bold(remediation)),
		}
		j.errlog.Add(err)
		return err
	}

	fmt.Fprintf(out, "Found %s at %s\n", j.packageExecutable, path)

	if j.validateScriptBuild {
		remediation := "npm run"
		pkgErr := fmt.Sprintf("package.json requires a `script` field with a `build` step defined that calls the `%s` binary", j.packageExecutable)
		remediation = fmt.Sprintf("Check your package.json has a `script` field with a `build` step defined:\n\n\t$ %s", text.Bold(remediation))

		// gosec flagged this:
		// G204 (CWE-78): Subprocess launched with variable
		// Disabling as the variables come from trusted sources:
		// The CLI parser enforces supported values via EnumVar.
		/* #nosec */
		cmd := exec.Command(j.toolchain, "run")
		stdoutStderr, err := cmd.CombinedOutput()
		if err != nil {
			j.errlog.Add(err)
			return fsterr.RemediationError{
				Inner:       fmt.Errorf("%s: %w", pkgErr, err),
				Remediation: remediation,
			}
		}

		if !strings.Contains(string(stdoutStderr), " build\n") {
			err := fsterr.RemediationError{
				Inner:       fmt.Errorf("%s:\n\n%s", pkgErr, stdoutStderr),
				Remediation: remediation,
			}
			j.errlog.Add(err)
			return err
		}
	}

	return nil
}

// Build implements the Toolchain interface and attempts to compile the package
// JavaScript source to a Wasm binary.
func (j JavaScript) Build(out, progress io.Writer, verbose bool) error {
	cmd := j.toolchain
	args := []string{"run", "build"}

	if j.build != "" {
		cmd, args = j.Shell.Build(j.build)
	}

	s := fstexec.Streaming{
		Command:  cmd,
		Args:     args,
		Env:      os.Environ(),
		Output:   out,
		Progress: progress,
		Verbose:  verbose,
	}
	if j.timeout > 0 {
		s.Timeout = time.Duration(j.timeout) * time.Second
	}
	if err := s.Exec(); err != nil {
		j.errlog.Add(err)
		return err
	}

	return nil
}
