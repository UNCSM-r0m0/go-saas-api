package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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

	http.HandleFunc("/execute", func(w http.ResponseWriter, r *http.Request) {
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

		// TODO: real sandboxed execution (firejail, nsjail, or container)
		// For now, return a stub response
		resp := executeResponse{
			Output: fmt.Sprintf("Executed %s code (%d chars). Sandbox not yet implemented.", req.Language, len(req.Code)),
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
