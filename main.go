package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func isValidIPv4(ip string) bool {
	return net.ParseIP(ip) != nil && strings.Count(ip, ":") < 1
}

func isValidDomain(domain string) bool {
	matched, _ := regexp.MatchString(`^([a-zA-Z0-9-]+\.)+[a-zA-Z]{2,}$`, domain)
	return matched && len(domain) <= 253
}

func runCommandBuffered(cmd *exec.Cmd, parser func(string) (map[string]interface{}, error)) ([]map[string]interface{}, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		out, err := parser(line)
		if err == nil && out != nil {
			results = append(results, out)
		}
	}

	if err := cmd.Wait(); err != nil {
		return nil, err
	}

	return results, nil
}

// Common function to run a command and stream its output parsed by a given parser
func streamCommand(w http.ResponseWriter, cmd *exec.Cmd, parser func(string) (map[string]interface{}, error)) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		return errors.New("streaming unsupported")
	}

	// Stream stdout
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			out, err := parser(line)
			if err == nil && out != nil {
				jsonLine, _ := json.Marshal(out)
				w.Write(jsonLine)
				w.Write([]byte("\n"))
				flusher.Flush()
			}
		}
	}()

	// Stream stderr as error JSON
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				errLine := map[string]string{"error": string(buf[:n])}
				json.NewEncoder(w).Encode(errLine)
				flusher.Flush()
			}
			if err != nil {
				break
			}
		}
	}()

	return cmd.Wait()
}

// Parser for mtr --raw output
func parseMTRRaw(line string) (map[string]interface{}, error) {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return nil, nil
	}

	switch parts[0] {
	case "x":
		if len(parts) >= 3 {
			return map[string]interface{}{
				"type":     "cycle",
				"cycle":    atoi(parts[1]),
				"cycle_id": atoi(parts[2]),
			}, nil
		}
	case "h":
		if len(parts) >= 3 {
			return map[string]interface{}{
				"type": "hop",
				"hop":  atoi(parts[1]),
				"ip":   parts[2],
			}, nil
		}
	case "p":
		if len(parts) >= 4 {
			return map[string]interface{}{
				"type":     "ping",
				"hop":      atoi(parts[1]),
				"rtt":      atoi(parts[2]),
				"cycle_id": atoi(parts[3]),
			}, nil
		}
	}
	return nil, nil
}

func parsePing(line string) (map[string]interface{}, error) {
	if strings.Contains(line, "bytes from") {
		fields := strings.Fields(line)
		var seq, timeMs string
		for _, f := range fields {
			if strings.HasPrefix(f, "icmp_seq=") {
				seq = strings.TrimPrefix(f, "icmp_seq=")
			} else if strings.HasPrefix(f, "seq=") { // <-- add this for BusyBox ping
				seq = strings.TrimPrefix(f, "seq=")
			} else if strings.HasPrefix(f, "time=") {
				// time=1.241 ms  -- remove " ms"
				timeMs = strings.TrimSuffix(strings.TrimPrefix(f, "time="), " ms")
				// Sometimes there might be no space, be safe:
				timeMs = strings.TrimSpace(timeMs)
			}
		}
		if seq != "" && timeMs != "" {
			rtt, err := strconv.ParseFloat(timeMs, 64)
			if err != nil {
				return nil, err
			}
			seqInt := atoi(seq)
			return map[string]interface{}{
				"type":    "ping",
				"seq":     seqInt,
				"rtt_ms":  rtt,
				"rawLine": line,
			}, nil
		}
	}
	return nil, nil
}

func validateTarget(target string) bool {
	return target != "" && (isValidIPv4(target) || isValidDomain(target))
}

func mtrHandler(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("target")
	if !validateTarget(target) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid target"})
		return
	}

	cmd := exec.Command("mtr", "--raw", "--no-dns", "--report-cycles", "10", target)
	streaming := r.URL.Query().Get("streaming") == "true"

	if streaming {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Transfer-Encoding", "chunked")
		if err := streamCommand(w, cmd, parseMTRRaw); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		}
	} else {
		results, err := runCommandBuffered(cmd, parseMTRRaw)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}

func pingHandler(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("target")
	if !validateTarget(target) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid target"})
		return
	}

	cmd := exec.Command("ping", "-c", "10", target)

	streaming := r.URL.Query().Get("streaming") == "true"

	if streaming {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Transfer-Encoding", "chunked")
		if err := streamCommand(w, cmd, parsePing); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
		}
	} else {
		results, err := runCommandBuffered(cmd, parsePing)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/mtr", mtrHandler)
	r.Get("/ping", pingHandler)

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	fmt.Println("Server running on http://localhost:8080")
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		fmt.Printf("Server error: %v\n", err)
	}
}
