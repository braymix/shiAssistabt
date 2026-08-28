package api

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

// chatClient forwards chat requests to the head's llama-server. There is no
// overall timeout because completions stream and can run long; the caller's
// request context still bounds the lifetime.
var chatClient = &http.Client{Timeout: 0}

// handleChat proxies an OpenAI-style chat request to whichever node is currently
// the head, so the built-in chat page (and any local tool) can talk to the mesh
// through this node without worrying about which device holds the endpoint or
// about browser CORS. Streaming (SSE) responses are passed straight through.
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	plan, ok := s.Plan()
	if !ok || plan.LLMURL == "" {
		writeError(w, http.StatusServiceUnavailable, "no cluster head yet — start the cluster first")
		return
	}
	target := strings.TrimRight(plan.LLMURL, "/") + "/chat/completions"
	proxyPost(w, r, target)
}

// proxyPost streams a POST from r to target and copies the response back,
// flushing as data arrives so server-sent events reach the browser live.
func proxyPost(w http.ResponseWriter, r *http.Request, target string) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, target, r.Body)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if a := r.Header.Get("Authorization"); a != "" {
		req.Header.Set("Authorization", a)
	}

	resp, err := chatClient.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "cluster head unreachable: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(resp.StatusCode)

	flusher, _ := w.(http.Flusher)
	buf := make([]byte, 4096)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if rerr != nil {
			return
		}
	}
}

// WebUIInfo tells the operator how to point Open WebUI at this mesh for a full
// chat + voice assistant.
type WebUIInfo struct {
	Ready     bool     `json:"ready"`
	Endpoint  string   `json:"endpoint"` // OpenAI-compatible base URL (…/v1)
	Docker    string   `json:"docker"`   // one-shot docker run command
	VoiceTips []string `json:"voice_tips"`
}

// handleWebUI returns a copy-pasteable Open WebUI setup for the current head.
func (s *Server) handleWebUI(w http.ResponseWriter, r *http.Request) {
	info := WebUIInfo{
		VoiceTips: []string{
			"Open WebUI ships Whisper STT built in: Admin → Settings → Audio → Speech-to-Text = Whisper (local).",
			"For speech-out pick a TTS engine under the same Audio panel (the built-in browser TTS works offline; Kokoro/Edge give nicer voices).",
			"Enable the microphone / call button in a chat to talk hands-free; on small clusters keep responses short for lower latency.",
		},
	}
	if plan, ok := s.Plan(); ok && plan.LLMURL != "" {
		info.Ready = true
		info.Endpoint = plan.LLMURL
		info.Docker = fmt.Sprintf(
			"docker run -d -p 3000:8080 "+
				"-e OPENAI_API_BASE_URL=%s -e OPENAI_API_KEY=sk-shika-local "+
				"-v open-webui:/app/backend/data --name open-webui --restart always "+
				"ghcr.io/open-webui/open-webui:main",
			plan.LLMURL)
	}
	writeJSON(w, info)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(code)
	_, _ = io.WriteString(w, fmt.Sprintf("{\n  \"error\": %q\n}\n", msg))
}
