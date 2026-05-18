package main

import (
	"os/exec"
	"strings"
	"testing"
)

func hasInterpreter(name string) bool {
	cmd := exec.Command(name, "-c", "print('ok')")
	return cmd.Run() == nil
}

func TestRunSandboxed_Python(t *testing.T) {
	if !hasInterpreter("python3") && !hasInterpreter("python") {
		t.Skip("python not installed")
	}
	output, err := runSandboxed("python", "print('hello from python')")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "hello from python") {
		t.Fatalf("expected 'hello from python', got: %s", output)
	}
}

func TestRunSandboxed_Javascript(t *testing.T) {
	if !hasInterpreter("node") {
		t.Skip("node not installed")
	}
	output, err := runSandboxed("javascript", "console.log('hello from js')")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "hello from js") {
		t.Fatalf("expected 'hello from js', got: %s", output)
	}
}

func TestRunSandboxed_Unsupported(t *testing.T) {
	_, err := runSandboxed("cobol", "DISPLAY 'HELLO'")
	if err == nil {
		t.Fatal("expected error for unsupported language")
	}
}

func TestRunSandboxed_Error(t *testing.T) {
	if !hasInterpreter("python3") && !hasInterpreter("python") {
		t.Skip("python not installed")
	}
	output, err := runSandboxed("python", "raise Exception('fail')")
	if err == nil {
		t.Fatal("expected error for failing code")
	}
	if !strings.Contains(output, "fail") && !strings.Contains(err.Error(), "fail") {
		t.Fatalf("expected error to contain 'fail', got output: %s, err: %v", output, err)
	}
}
