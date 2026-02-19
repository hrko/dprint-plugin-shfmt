//go:build integration

package integration

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const libunwindNoise = "libunwind: __unw_add_dynamic_fde: bad fde: FDE is really a CIE"

type integrationRunner struct {
	repoRoot string
	tempRoot string
	wasmPath string
	env      []string
}

type fmtRunResult struct {
	exitCode    int
	stdout      string
	cleanStderr string
}

type integrationCase struct {
	name           string
	fixture        string
	virtualPath    string
	exitCode       int
	stderrContains []string
	repeat         int
}

type caseFixture struct {
	configPath     string
	input          string
	expectedStdout string
}

func TestDprintPluginIntegration(t *testing.T) {
	runner := newIntegrationRunner(t)

	cases := []integrationCase{
		{name: "format-success"},
		{name: "no-change"},
		{name: "parse-error", exitCode: 1, stderrContains: []string{"must end with \"fi\""}},
		{name: "variant-sh-fails-for-bash-array", virtualPath: "sample.sh", exitCode: 1, stderrContains: []string{"arrays are a bash/mksh feature"}},
		{name: "variant-bash-succeeds-for-bash-array", virtualPath: "sample.bash"},
		{name: "shebang-precedence"},
		{name: "global-override"},
		{name: "comment-and-shebang-preservation"},
		{name: "use-tabs-option"},
		{name: "binary-next-line-option"},
		{name: "switch-case-indent-option"},
		{name: "space-redirects-option"},
		{name: "func-next-line-option"},
		{name: "minify-option"},
		{name: "config-type-error-diagnostic", exitCode: 1, stderrContains: []string{"Expected 'funcNextLine' to be a boolean", "Had 1 configuration errors."}},
		{name: "unknown-property-diagnostic", exitCode: 1, stderrContains: []string{"Unknown property 'unknownField'.", "Had 1 configuration errors."}},
		{name: "repeated-invocations-same-cache", repeat: 3},
	}

	for _, tc := range cases {
		tc := withCaseDefaults(tc)
		t.Run(tc.name, func(t *testing.T) {
			runner.runCase(t, tc)
		})
	}
}

func withCaseDefaults(tc integrationCase) integrationCase {
	if tc.fixture == "" {
		tc.fixture = tc.name
	}
	if tc.virtualPath == "" {
		tc.virtualPath = "sample.sh"
	}
	if tc.repeat <= 0 {
		tc.repeat = 1
	}
	return tc
}

func loadFixture(t *testing.T, repoRoot string, fixtureName string) caseFixture {
	t.Helper()

	fixtureDir := filepath.Join(repoRoot, "integration", "testdata", "cases", fixtureName)
	configPath := filepath.Join(fixtureDir, "config.json")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("%s: missing config.json: %v", fixtureName, err)
	}

	inputBytes, err := os.ReadFile(filepath.Join(fixtureDir, "input.sh"))
	if err != nil {
		t.Fatalf("%s: failed to read input.sh: %v", fixtureName, err)
	}

	expectedStdoutBytes, err := os.ReadFile(filepath.Join(fixtureDir, "expected.stdout"))
	if err != nil {
		t.Fatalf("%s: failed to read expected.stdout: %v", fixtureName, err)
	}

	return caseFixture{
		configPath:     configPath,
		input:          string(inputBytes),
		expectedStdout: string(expectedStdoutBytes),
	}
}

func newIntegrationRunner(t *testing.T) *integrationRunner {
	t.Helper()

	repoRoot := repoRootFromCaller(t)
	tempRoot := t.TempDir()
	cacheRoot := filepath.Join(tempRoot, "cache")
	dprintCache := filepath.Join(tempRoot, "dprint-cache")
	tempDir := filepath.Join(tempRoot, "tmp")
	wasmPath := filepath.Join(tempRoot, "plugin-integration.wasm")

	for _, dirPath := range []string{cacheRoot, dprintCache, tempDir} {
		if err := os.MkdirAll(dirPath, 0o755); err != nil {
			t.Fatalf("failed to create directory %s: %v", dirPath, err)
		}
	}

	runner := &integrationRunner{
		repoRoot: repoRoot,
		tempRoot: tempRoot,
		wasmPath: wasmPath,
		env: append(
			os.Environ(),
			"XDG_CACHE_HOME="+cacheRoot,
			"DPRINT_CACHE_DIR="+dprintCache,
			"TMPDIR="+tempDir,
			"TMP="+tempDir,
			"TEMP="+tempDir,
		),
	}
	runner.buildWasm(t)

	return runner
}

func repoRootFromCaller(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to determine test source location")
	}

	return filepath.Dir(filepath.Dir(filename))
}

func (r *integrationRunner) buildWasm(t *testing.T) {
	t.Helper()

	cmd := exec.Command(
		"tinygo",
		"build",
		"-o", r.wasmPath,
		"-target=wasm-unknown",
		"-scheduler=none",
		"-panic=trap",
		"-no-debug",
		r.repoRoot,
	)
	cmd.Dir = r.repoRoot
	cmd.Env = r.env

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build plugin.wasm: %v\n%s", err, string(output))
	}
}

func (r *integrationRunner) runCase(t *testing.T, tc integrationCase) {
	t.Helper()

	fixture := loadFixture(t, r.repoRoot, tc.fixture)

	for i := 0; i < tc.repeat; i++ {
		result := r.runFmt(t, fixture.configPath, tc.virtualPath, fixture.input)
		label := fmt.Sprintf("%s run %d", tc.name, i+1)
		assertCaseResult(t, label, tc, fixture, result)
	}
}

func (r *integrationRunner) runFmt(t *testing.T, configPath string, virtualPath string, input string) fmtRunResult {
	t.Helper()

	cmd := exec.Command(
		"dprint",
		"fmt",
		"--log-level", "info",
		"--config", configPath,
		"--plugins", r.wasmPath,
		"--stdin", virtualPath,
	)
	cmd.Dir = r.repoRoot
	cmd.Env = r.env
	cmd.Stdin = strings.NewReader(input)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(runErr, &exitErr) {
			t.Fatalf("failed to run dprint fmt: %v", runErr)
		}
		exitCode = exitErr.ExitCode()
	}

	return fmtRunResult{
		exitCode:    exitCode,
		stdout:      stdout.String(),
		cleanStderr: cleanStderr(stderr.String()),
	}
}

func cleanStderr(raw string) string {
	lines := strings.Split(raw, "\n")
	filtered := make([]string, 0, len(lines))

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, "Compiling ") {
			continue
		}
		if line == libunwindNoise {
			continue
		}
		filtered = append(filtered, line)
	}

	return strings.Join(filtered, "\n")
}

func assertCaseResult(t *testing.T, label string, tc integrationCase, fixture caseFixture, result fmtRunResult) {
	t.Helper()

	if result.exitCode != tc.exitCode {
		t.Fatalf("%s: expected exit code %d, got %d\nstderr:\n%s", label, tc.exitCode, result.exitCode, result.cleanStderr)
	}

	if result.stdout != fixture.expectedStdout {
		t.Fatalf("%s: unexpected stdout\nexpected:\n%s\nactual:\n%s", label, fixture.expectedStdout, result.stdout)
	}

	if len(tc.stderrContains) == 0 {
		if result.cleanStderr != "" {
			t.Fatalf("%s: expected empty stderr, got:\n%s", label, result.cleanStderr)
		}
		return
	}

	for _, expected := range tc.stderrContains {
		if !strings.Contains(result.cleanStderr, expected) {
			t.Fatalf("%s: expected stderr to contain %q, got:\n%s", label, expected, result.cleanStderr)
		}
	}
}
