package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// RegisterProcessAPI registers endpoints to start, stop and check status of the picoclaw gateway.
func RegisterProcessAPI(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/process/status", handleStatusGateway)
	mux.HandleFunc("POST /api/process/start", handleStartGateway)
	mux.HandleFunc("POST /api/process/stop", handleStopGateway)
}

func handleStartGateway(w http.ResponseWriter, r *http.Request) {
	// Locate picoclaw executable:
	// 1. Try same directory as current executable
	// 2. Fallback to just "picoclaw" (relies on $PATH)
	execPath := "picoclaw"

	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidate := filepath.Join(dir, "picoclaw")
		if runtime.GOOS == "windows" {
			candidate += ".exe"
		}

		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			execPath = candidate
		}
	}

	cmd := exec.Command(execPath, "gateway")
	if err := cmd.Start(); err != nil {
		log.Printf("Failed to start picoclaw gateway: %v\n", err)
		http.Error(w, fmt.Sprintf("Failed to start gateway: %v", err), http.StatusInternalServerError)
		return
	}

	// We don't Wait() because we want it to run in the background
	// You may want to detach it from the parent process properly on Windows
	log.Printf("Started picoclaw gateway (PID: %d) from %s\n", cmd.Process.Pid, execPath)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status": "ok",
		"pid":    cmd.Process.Pid,
	})
}

func handleStopGateway(w http.ResponseWriter, r *http.Request) {
	var err error
	if runtime.GOOS == "windows" {
		// Kill via taskkill finding picoclaw.exe (though it might kill this config tool if it's named picoclaw-config.exe...? No, /IM does exact match usually, but just to be safe let's stop exactly picoclaw.exe)
		// Alternatively, we use powershell to kill processes with commandline containing 'gateway'
		psCmd := `Get-WmiObject Win32_Process | Where-Object { $_.CommandLine -match 'picoclaw.*gateway' } | ForEach-Object { Stop-Process $_.ProcessId -Force }`
		err = exec.Command("powershell", "-Command", psCmd).Run()
	} else {
		// Linux/macOS
		err = exec.Command("pkill", "-f", "picoclaw gateway").Run()
	}

	if err != nil {
		log.Printf("Warning: Failed to stop gateway (perhaps not running?): %v\n", err)
		// We still return 200 OK because pkill returns an error if no process was found
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status": "ok", // or "not_found"
			"msg":    "Stop command executed, but returned error (process might not be running).",
			"error":  err.Error(),
		})
		return
	}

	log.Printf("Stopped picoclaw gateway processes.\n")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

func handleStatusGateway(w http.ResponseWriter, r *http.Request) {
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://localhost:18790/health")

	w.Header().Set("Content-Type", "application/json")

	if err != nil {
		// If we cannot reach the gateway health endpoint, we assume it is stopped
		json.NewEncoder(w).Encode(map[string]any{
			"process_status": "stopped",
			"error":          err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		json.NewEncoder(w).Encode(map[string]any{
			"process_status": "error",
			"status_code":    resp.StatusCode,
		})
		return
	}

	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		json.NewEncoder(w).Encode(map[string]any{
			"process_status": "error",
			"error":          "invalid response from gateway",
		})
		return
	}

	// Gateway is running and responded properly
	data["process_status"] = "running"
	json.NewEncoder(w).Encode(data)
}
