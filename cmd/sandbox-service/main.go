package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type executeRequest struct {
	Code     string `json:"code"`
	Language string `json:"language"`
}

type executeResponse struct {
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3006"
	}

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","service":"sandbox-service"}`))
	})

	http.HandleFunc("/sandbox/execute", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req executeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(executeResponse{Error: "invalid request"})
			return
		}

		output, err := runSandboxed(req.Language, req.Code)
		resp := executeResponse{Output: output}
		if err != nil {
			resp.Error = err.Error()
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	})

	fmt.Printf("sandbox-service listening on :%s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}

func findPython() string {
	for _, cmd := range []string{"python3", "python"} {
		if out, err := exec.Command(cmd, "--version").CombinedOutput(); err == nil && len(out) > 0 {
			return cmd
		}
	}
	return "python"
}

func runSandboxed(language, code string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "sandbox-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	var filename string
	var args []string

	switch strings.ToLower(language) {
	case "python", "py":
		filename = "main.py"
		args = []string{findPython(), filename}
	case "javascript", "js", "node":
		filename = "main.js"
		args = []string{"node", filename}
	case "go", "golang":
		filename = "main.go"
		args = []string{"go", "run", filename}
	case "shell", "bash", "sh":
		filename = "script.sh"
		args = []string{"bash", filename}
	default:
		return "", fmt.Errorf("unsupported language: %s", language)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, filename), []byte(code), 0644); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = tmpDir
	cmd.Env = []string{"PATH=" + os.Getenv("PATH")}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return stdout.String(), fmt.Errorf("execution timed out (10s)")
		}
		return stdout.String(), fmt.Errorf("%v: %s", err, stderr.String())
	}

	result := stdout.String()
	if stderr.Len() > 0 {
		result += "\n[stderr]:\n" + stderr.String()
	}

	// Limit output size
	const maxOutput = 128 * 1024
	if len(result) > maxOutput {
		result = result[:maxOutput] + "\n...[output truncated]"
	}

	return result, nil
}
