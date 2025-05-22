package main_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

var (
	pathToBinary string
	projectRoot  string
)

func TestMain(m *testing.M) {
	// Determine project root dynamically. Assumes tests are run from within the project structure.
	// This navigates up from "cmd/gitlabviz" (where the test file is) to the project root.
	// Adjust depth if test file location changes.
	var err error
	projectRoot, err = getProjectRoot()
	if err != nil {
		fmt.Printf("Failed to get project root: %v\n", err)
		os.Exit(1)
	}

	// Build the gitlabviz binary
	// Output to a temporary location or a known build directory like 'bin'
	// For simplicity, placing it in a 'bin' directory in project root.
	// Ensure .gitignore handles 'bin/' if it's not already.
	tempBinDir := filepath.Join(projectRoot, "bin")
	if err := os.MkdirAll(tempBinDir, 0755); err != nil {
		fmt.Printf("Failed to create temp bin directory: %v\n", err)
		os.Exit(1)
	}
	pathToBinary = filepath.Join(tempBinDir, "gitlabviz_test_runner")
	if //goland:noinspection GoBoolExpressions
	runtime.GOOS == "windows" {
		pathToBinary += ".exe"
	}

	fmt.Printf("Building test binary at: %s\n", pathToBinary)
	// Build command needs to be run from the main package directory of the CLI tool
	buildCmd := exec.Command("go", "build", "-o", pathToBinary, ".")
	buildCmd.Dir = filepath.Join(projectRoot, "cmd", "gitlabviz") // Assuming main.go is in cmd/gitlabviz
	buildOutput, err := buildCmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Failed to build gitlabviz binary: %v\nOutput: %s\n", err, string(buildOutput))
		os.Exit(1)
	}

	// Run tests
	exitCode := m.Run()

	// Clean up
	err = os.Remove(pathToBinary)
	if err != nil {
		fmt.Printf("Failed to remove test binary: %v\n", err)
	}
	// Attempt to remove bin dir if empty, not critical if it fails
	_ = os.Remove(tempBinDir)


	os.Exit(exitCode)
}

// getProjectRoot tries to find the project root by looking for go.mod
func getProjectRoot() (string, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < 5; i++ { // Limit depth to avoid infinite loops
		if _, err := os.Stat(filepath.Join(currentDir, "go.mod")); err == nil {
			return currentDir, nil
		}
		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir { // Reached root of filesystem
			break
		}
		currentDir = parentDir
	}
	return "", fmt.Errorf("could not find project root (go.mod)")
}


func runGitlabvizCommand(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	cmdArgs := append([]string{"--no-color"}, args...) // Prepend no-color for consistent output
	cmd := exec.Command(pathToBinary, cmdArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run() // Use Run for most cases, as ExitError is what we check for errors

	stdoutStr := stdout.String()
	stderrStr := stderr.String()

	// Log output for debugging, especially on test failure
	if t.Failed() || err != nil {
		t.Logf("Command: %s %s", pathToBinary, strings.Join(cmdArgs, " "))
		t.Logf("Stdout: %s", stdoutStr)
		t.Logf("Stderr: %s", stderrStr)
		if err != nil {
			t.Logf("Error: %v", err)
		}
	}
	return stdoutStr, stderrStr, err
}

func TestTemplateCommand_BasicIncludeAndMerge(t *testing.T) {
	exampleFilePath := filepath.Join(projectRoot, "examples", "include_main.yml")
	stdout, stderr, err := runGitlabvizCommand(t, "template", exampleFilePath)

	assert.NoError(t, err, "Running template command should not produce an error")
	assert.Empty(t, stderr, "Stderr should be empty on successful run")

	var output map[string]interface{}
	err = yaml.Unmarshal([]byte(stdout), &output)
	assert.NoError(t, err, "Failed to unmarshal YAML output")

	// Variables
	variables, ok := output["variables"].(map[string]interface{})
	assert.True(t, ok, "Output should contain a 'variables' map")
	assert.Equal(t, "overridden_base_value", variables["BASE_VAR"], "BASE_VAR should be overridden")
	assert.Equal(t, "main_value", variables["MAIN_VAR"], "MAIN_VAR should be present")

	// Stages
	// Based on current merge logic: main's stages `[test, build]` followed by unique stages from `include_base.yml` `[deploy]`
	expectedStages := []interface{}{"test", "build", "deploy"}
	stages, ok := output["stages"].([]interface{})
	assert.True(t, ok, "Output should contain a 'stages' list")
	assert.Equal(t, expectedStages, stages, "Stages are not correctly merged")

	// Jobs
	_, mainJobExists := output["main_job"].(map[string]interface{})
	assert.True(t, mainJobExists, "main_job should exist")

	baseJob, baseJobExists := output["base_job"].(map[string]interface{})
	assert.True(t, baseJobExists, "base_job should exist")
	baseJobScript, ok := baseJob["script"].([]interface{}) // Script is parsed as []interface{} by yaml.v3 from []string
	assert.True(t, ok, "base_job script should be a list")
	assert.Contains(t, baseJobScript, "Overridden base job script in main", "base_job script is not overridden")
	// Also check that the original base_job stage is preserved if not overridden,
	// or updated if overridden. Here, 'base_job' in include_main.yml doesn't specify a stage,
	// so it should pick up the stage from include_base.yml.
	assert.Equal(t, "build", baseJob["stage"], "base_job stage is not correct")

}

func TestTemplateCommand_NestedIncludes(t *testing.T) {
	exampleFilePath := filepath.Join(projectRoot, "examples", "include_nested_main.yml")
	stdout, stderr, err := runGitlabvizCommand(t, "template", exampleFilePath)

	assert.NoError(t, err, "Running template command with nested includes should not produce an error")
	assert.Empty(t, stderr, "Stderr should be empty on successful run with nested includes")

	var output map[string]interface{}
	err = yaml.Unmarshal([]byte(stdout), &output)
	assert.NoError(t, err, "Failed to unmarshal YAML output for nested includes")

	_, mainJobExists := output["main_nested_job"].(map[string]interface{})
	assert.True(t, mainJobExists, "main_nested_job should exist")

	_, level1JobExists := output["level1_job"].(map[string]interface{})
	assert.True(t, level1JobExists, "level1_job should exist from nested include")

	_, level2JobExists := output["level2_job"].(map[string]interface{})
	assert.True(t, level2JobExists, "level2_job should exist from doubly nested include")
}

func TestTemplateCommand_CircularIncludeDetection(t *testing.T) {
	exampleFilePath := filepath.Join(projectRoot, "examples", "include_circular_1.yml")
	_, stderr, err := runGitlabvizCommand(t, "template", exampleFilePath)

	assert.Error(t, err, "Running template command with circular includes should produce an error")
	// Check if the error is an ExitError, indicating the command ran and exited with non-zero status
	_, ok := err.(*exec.ExitError)
	assert.True(t, ok, "Error should be an exec.ExitError for failed command execution")

	assert.Contains(t, stderr, "circular dependency detected", "Stderr should report circular dependency")
}

// Add a placeholder for runtime import if GoLand complains, or ensure it's there.
// It's used in TestMain for OS-specific binary naming.
import "runtime"
