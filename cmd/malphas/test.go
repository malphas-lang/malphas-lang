package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/malphas-lang/malphas-lang/internal/ast"
	"github.com/malphas-lang/malphas-lang/internal/parser"
	"github.com/malphas-lang/malphas-lang/internal/types"
)

// TestResult represents the result of running a single test
type TestResult struct {
	Name     string
	Passed   bool
	Error    error
	Output   string
	Duration string // Could add timing later
}

// runTest executes the test command
func runTest(args []string) {
	if len(args) == 0 {
		// Run all tests in current directory and subdirectories
		runAllTests(".")
	} else {
		// Run tests for specified path(s)
		for _, arg := range args {
			runAllTests(arg)
		}
	}
}

// runAllTests discovers and runs all tests in the given directory or file
func runAllTests(path string) {
	var testFiles []string

	// Check if path is a file or directory
	info, err := os.Stat(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error accessing path %s: %v\n", path, err)
		os.Exit(1)
	}

	if info.IsDir() {
		// Find all test files in directory
		var err error
		testFiles, err = findTestFiles(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error finding test files: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Single file - check if it's a test file
		if strings.HasSuffix(path, "_test.mal") || strings.HasSuffix(path, ".mal") {
			testFiles = []string{path}
		}
	}

	if len(testFiles) == 0 {
		fmt.Printf("No test files found in %s\n", path)
		return
	}

	fmt.Printf("Running tests in %s...\n\n", path)

	var totalTests int
	var passedTests int
	var failedTests int

	// Run each test file
	for _, testFile := range testFiles {
		results := runTestFile(testFile)
		for _, result := range results {
			totalTests++
			if result.Passed {
				passedTests++
				fmt.Printf("  ✓ %s\n", result.Name)
			} else {
				failedTests++
				fmt.Printf("  ✗ %s\n", result.Name)
				if result.Error != nil {
					fmt.Printf("    Error: %v\n", result.Error)
				}
				if result.Output != "" {
					fmt.Printf("    Output: %s\n", result.Output)
				}
			}
		}
	}

	// Print summary
	fmt.Printf("\n")
	fmt.Printf("Test Results: %d total, %d passed, %d failed\n", totalTests, passedTests, failedTests)

	if failedTests > 0 {
		os.Exit(1)
	}
}

// findTestFiles finds all test files in the given directory
// Test files are those ending with _test.mal or in a tests/ directory
func findTestFiles(dir string) ([]string, error) {
	var testFiles []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories
		if info.IsDir() && strings.HasPrefix(info.Name(), ".") {
			return filepath.SkipDir
		}

		// Check if it's a test file
		if !info.IsDir() {
			// Test files are those ending with _test.mal
			if strings.HasSuffix(path, "_test.mal") {
				testFiles = append(testFiles, path)
			} else if strings.HasSuffix(path, ".mal") {
				// Also check if we're in a tests/ directory (but not already a _test.mal file)
				dir := filepath.Dir(path)
				if filepath.Base(dir) == "tests" {
					testFiles = append(testFiles, path)
				}
			}
		}

		return nil
	})

	return testFiles, err
}

// runTestFile runs all tests in a single test file
func runTestFile(filename string) []TestResult {
	// Read and parse the test file
	src, err := os.ReadFile(filename)
	if err != nil {
		return []TestResult{
			{
				Name:   filepath.Base(filename),
				Passed: false,
				Error:  fmt.Errorf("failed to read file: %v", err),
			},
		}
	}

	p := parser.New(string(src), parser.WithFilename(filename))
	file := p.ParseFile()

	if len(p.Errors()) > 0 {
		return []TestResult{
			{
				Name:   filepath.Base(filename),
				Passed: false,
				Error:  fmt.Errorf("parse errors: %v", p.Errors()),
			},
		}
	}

	// Type check
	checker := types.NewChecker()
	absFilename, err := filepath.Abs(filename)
	if err != nil {
		absFilename = filename
	}
	checker.CheckWithFilename(file, absFilename)

	if len(checker.Errors) > 0 {
		return []TestResult{
			{
				Name:   filepath.Base(filename),
				Passed: false,
				Error:  fmt.Errorf("type check errors: %v", checker.Errors),
			},
		}
	}

	// Find all test functions (functions starting with "test_")
	testFunctions := findTestFunctions(file)

	if len(testFunctions) == 0 {
		// If no test functions found, check if there's a main function
		// If so, treat the file as a single test
		hasMain := false
		for _, decl := range file.Decls {
			if fnDecl, ok := decl.(*ast.FnDecl); ok {
				if fnDecl.Name != nil && fnDecl.Name.Name == "main" {
					hasMain = true
					break
				}
			}
		}
		if hasMain {
			return []TestResult{
				runSingleTest(filename, file, checker, filepath.Base(filename)),
			}
		}
		// No test functions and no main - skip this file
		return []TestResult{}
	}

	// If we have test functions, we need to run each one individually
	// For now, we'll run the file once and expect main() to call all test functions
	// If main() exists and calls the tests, we report based on exit code
	// If main() doesn't exist or doesn't call tests, we could generate a test harness
	// For simplicity, we'll require main() to exist and call all test functions

	// Check if main() exists
	hasMain := false
	for _, decl := range file.Decls {
		if fnDecl, ok := decl.(*ast.FnDecl); ok {
			if fnDecl.Name != nil && fnDecl.Name.Name == "main" {
				hasMain = true
				break
			}
		}
	}

	if !hasMain {
		// No main() - can't run tests individually yet
		// In the future, we could generate a test harness
		return []TestResult{
			{
				Name:   filepath.Base(filename),
				Passed: false,
				Error:  fmt.Errorf("test file must have a main() function that calls test functions"),
			},
		}
	}

	// Run the test file once - main() should call all test functions
	// If any test fails (panics or returns error), the whole file fails
	result := runSingleTest(filename, file, checker, filepath.Base(filename))

	// Report the same result for all test functions
	// This is a limitation: we can't tell which specific test failed
	// TODO: Generate a test harness that calls each test function individually
	var results []TestResult
	for _, fn := range testFunctions {
		results = append(results, TestResult{
			Name:   fn.Name.Name,
			Passed: result.Passed,
			Error:  result.Error,
			Output: result.Output,
		})
	}

	return results
}

// findTestFunctions finds all functions that start with "test_"
func findTestFunctions(file *ast.File) []*ast.FnDecl {
	var testFunctions []*ast.FnDecl

	for _, decl := range file.Decls {
		if fnDecl, ok := decl.(*ast.FnDecl); ok {
			if fnDecl.Name != nil && strings.HasPrefix(fnDecl.Name.Name, "test_") {
				testFunctions = append(testFunctions, fnDecl)
			}
		}
	}

	return testFunctions
}

// runSingleTest runs a single test by compiling and executing it
func runSingleTest(filename string, file *ast.File, checker *types.Checker, testName string) TestResult {
	// Compile the test file to LLVM IR (file and checker are already parsed/checked)
	irFile, err := compileToLLVM(file, checker)
	if err != nil {
		return TestResult{
			Name:   testName,
			Passed: false,
			Error:  fmt.Errorf("compilation failed: %v", err),
		}
	}
	defer os.Remove(irFile) // Clean up temp file

	// Find llc
	llcPath, err := findLLC()
	if err != nil {
		return TestResult{
			Name:   testName,
			Passed: false,
			Error:  fmt.Errorf("llc not found: %v", err),
		}
	}

	// Compile to object file
	objFile := irFile + ".o"
	cmd := exec.Command(llcPath, "-filetype=obj", "-mtriple=arm64-apple-darwin", "-o", objFile, irFile)
	var llcStderr strings.Builder
	cmd.Stderr = &llcStderr
	if err := cmd.Run(); err != nil {
		return TestResult{
			Name:   testName,
			Passed: false,
			Error:  fmt.Errorf("llc failed: %v\n%s", err, llcStderr.String()),
		}
	}
	defer os.Remove(objFile)

	// Create temp executable
	exeFile, err := os.CreateTemp("", "malphas_test_*.exe")
	if err != nil {
		return TestResult{
			Name:   testName,
			Passed: false,
			Error:  fmt.Errorf("failed to create temp file: %v", err),
		}
	}
	exeFile.Close()
	exePath := exeFile.Name()
	defer os.Remove(exePath)

	// Find runtime object file
	runtimeObj := filepath.Join("runtime", "runtime.o")
	if _, err := os.Stat(runtimeObj); os.IsNotExist(err) {
		// Try alternative paths
		runtimeObj = filepath.Join(filepath.Dir(filename), "..", "runtime", "runtime.o")
		if _, err := os.Stat(runtimeObj); os.IsNotExist(err) {
			runtimeObj = filepath.Join("..", "runtime", "runtime.o")
		}
	}

	// Link with runtime
	linkArgs := []string{"-o", exePath, objFile}
	if _, err := os.Stat(runtimeObj); err == nil {
		linkArgs = append(linkArgs, runtimeObj)
	}

	// Add library path for libgc if needed
	if brewPrefix := os.Getenv("HOMEBREW_PREFIX"); brewPrefix != "" {
		linkArgs = append(linkArgs, "-L"+brewPrefix+"/lib")
	} else {
		for _, prefix := range []string{"/opt/homebrew", "/usr/local"} {
			if _, err := os.Stat(prefix + "/lib/libgc.a"); err == nil {
				linkArgs = append(linkArgs, "-L"+prefix+"/lib")
				break
			}
		}
	}

	linkArgs = append(linkArgs, "-lgc", "-pthread")

	linkCmd := exec.Command("clang", linkArgs...)
	var linkStderr strings.Builder
	linkCmd.Stderr = &linkStderr
	if err := linkCmd.Run(); err != nil {
		return TestResult{
			Name:   testName,
			Passed: false,
			Error:  fmt.Errorf("linking failed: %v\n%s", err, linkStderr.String()),
		}
	}

	// Run the test
	runCmd := exec.Command(exePath)
	output, err := runCmd.CombinedOutput()

	// Check exit code - test passes if exit code is 0
	passed := err == nil
	if err != nil {
		// Check if it's an exit error with non-zero code
		if exitErr, ok := err.(*exec.ExitError); ok {
			passed = exitErr.ExitCode() == 0
		}
	}

	return TestResult{
		Name:   testName,
		Passed: passed,
		Error:  err,
		Output: strings.TrimSpace(string(output)),
	}
}
