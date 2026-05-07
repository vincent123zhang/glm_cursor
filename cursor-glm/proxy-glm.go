package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"golang.org/x/net/http2"
)

const (
	defaultGLMBaseURL  = "https://open.bigmodel.cn/api/paas/v4"
	defaultGLMModel    = "glm-5.1"
	defaultCursorModel = "gpt-4o"
)

var (
	glmAPIKey       string
	glmBaseURL      string
	glmModel        string
	cursorModel     string
	proxyAPIKey     string
	proxyListenAddr string
)

type ModelsResponse struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

func init() {
	if err := godotenv.Load(".env", ".env.glm"); err != nil {
		log.Printf("Warning: .env/.env.glm file not found or error loading it: %v", err)
	}

	glmAPIKey = strings.TrimSpace(os.Getenv("GLM_API_KEY"))
	if glmAPIKey == "" {
		log.Fatal("GLM_API_KEY environment variable is required")
	}

	glmBaseURL = strings.TrimRight(strings.TrimSpace(os.Getenv("GLM_BASE_URL")), "/")
	if glmBaseURL == "" {
		glmBaseURL = defaultGLMBaseURL
	}

	glmModel = strings.TrimSpace(os.Getenv("GLM_MODEL"))
	if glmModel == "" {
		glmModel = defaultGLMModel
	}

	cursorModel = strings.TrimSpace(os.Getenv("GLM_CURSOR_MODEL"))
	if cursorModel == "" {
		cursorModel = defaultCursorModel
	}

	proxyAPIKey = strings.TrimSpace(os.Getenv("GLM_PROXY_API_KEY"))
	if proxyAPIKey == "" {
		proxyAPIKey = glmAPIKey
	}

	proxyListenAddr = strings.TrimSpace(os.Getenv("GLM_PROXY_ADDR"))
	if proxyListenAddr == "" {
		proxyListenAddr = ":9000"
	}

	log.Printf("Initialized GLM proxy with upstream model: %s, cursor model: %s, endpoint: %s", glmModel, cursorModel, glmBaseURL)
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)

	server := &http.Server{
		Addr:    proxyListenAddr,
		Handler: http.HandlerFunc(proxyHandler),
	}

	if err := http2.ConfigureServer(server, &http2.Server{}); err != nil {
		log.Fatalf("Failed to configure HTTP/2: %v", err)
	}

	log.Printf("Starting GLM proxy server on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(w)

	if r.Method == http.MethodOptions {
		return
	}

	if !isAuthorized(r) {
		http.Error(w, "Missing or invalid Authorization header", http.StatusUnauthorized)
		return
	}

	switch {
	case isModelsPath(r.URL.Path) && r.Method == http.MethodGet:
		handleModelsRequest(w)
		return
	case isChatCompletionsPath(r.URL.Path) && r.Method == http.MethodPost:
		handleChatCompletions(w, r)
		return
	default:
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
}

func enableCors(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Length")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
}

func isAuthorized(r *http.Request) bool {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return false
	}
	return strings.TrimPrefix(authHeader, "Bearer ") == proxyAPIKey
}

func isModelsPath(path string) bool {
	return path == "/v1/models" || path == "/models"
}

func isChatCompletionsPath(path string) bool {
	return path == "/v1/chat/completions" || path == "/chat/completions"
}

func handleModelsRequest(w http.ResponseWriter) {
	response := ModelsResponse{
		Object: "list",
		Data: []Model{
			{
				ID:      cursorModel,
				Object:  "model",
				Created: time.Now().Unix(),
				OwnedBy: "bigmodel",
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error writing models response: %v", err)
	}
}

func handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request", http.StatusBadRequest)
		return
	}

	modifiedBody, stream, err := buildGLMRequestBody(body)
	if err != nil {
		log.Printf("Error parsing request JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	targetURL := glmBaseURL + "/chat/completions"
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	proxyReq, err := http.NewRequest(r.Method, targetURL, bytes.NewReader(modifiedBody))
	if err != nil {
		log.Printf("Error creating proxy request: %v", err)
		http.Error(w, "Error creating proxy request", http.StatusInternalServerError)
		return
	}

	copyHeaders(proxyReq.Header, r.Header)
	proxyReq.Header.Set("Authorization", "Bearer "+glmAPIKey)
	proxyReq.Header.Set("Content-Type", "application/json")
	proxyReq.Header.Set("Accept-Encoding", "identity")
	if stream {
		proxyReq.Header.Set("Accept", "text/event-stream")
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	if stream {
		client.Timeout = 0
	}

	resp, err := client.Do(proxyReq)
	if err != nil {
		log.Printf("Error forwarding request: %v", err)
		http.Error(w, "Error forwarding request", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	copyResponseHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)

	if stream {
		streamResponse(w, resp.Body)
		return
	}

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error writing response: %v", err)
		return
	}

	if resp.StatusCode < http.StatusBadRequest {
		body = rewriteResponseModel(body, cursorModel)
	}

	if _, err := w.Write(body); err != nil {
		log.Printf("Error writing response: %v", err)
	}
}

func buildGLMRequestBody(body []byte) ([]byte, bool, error) {
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, false, err
	}

	payload["model"] = glmModel
	normalizeMessages(payload)
	convertLegacyFunctions(payload)

	stream, _ := payload["stream"].(bool)
	modifiedBody, err := json.Marshal(payload)
	if err != nil {
		return nil, false, fmt.Errorf("marshal modified request: %w", err)
	}
	return modifiedBody, stream, nil
}

func normalizeMessages(payload map[string]interface{}) {
	rawMessages, ok := payload["messages"].([]interface{})
	if !ok {
		return
	}

	for _, rawMessage := range rawMessages {
		message, ok := rawMessage.(map[string]interface{})
		if !ok {
			continue
		}

		parts, ok := message["content"].([]interface{})
		if !ok {
			continue
		}

		var textParts []string
		for _, rawPart := range parts {
			part, ok := rawPart.(map[string]interface{})
			if !ok || part["type"] != "text" {
				continue
			}
			if text, ok := part["text"].(string); ok && text != "" {
				textParts = append(textParts, text)
			}
		}
		message["content"] = strings.Join(textParts, "\n")
	}
}

func convertLegacyFunctions(payload map[string]interface{}) {
	if _, hasTools := payload["tools"]; hasTools {
		return
	}

	functions, ok := payload["functions"].([]interface{})
	if !ok || len(functions) == 0 {
		return
	}

	tools := make([]interface{}, 0, len(functions))
	for _, fn := range functions {
		tools = append(tools, map[string]interface{}{
			"type":     "function",
			"function": fn,
		})
	}
	payload["tools"] = tools
	delete(payload, "functions")
}

func streamResponse(w http.ResponseWriter, body io.Reader) {
	flusher, _ := w.(http.Flusher)
	reader := bufio.NewReader(body)

	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			line = rewriteStreamLineModel(line, cursorModel)
			if _, writeErr := w.Write(line); writeErr != nil {
				log.Printf("Error writing stream response: %v", writeErr)
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if err != nil {
			if err != io.EOF {
				log.Printf("Error reading stream response: %v", err)
			}
			return
		}
	}
}

func rewriteResponseModel(body []byte, model string) []byte {
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return body
	}
	payload["model"] = model
	modifiedBody, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return modifiedBody
}

func rewriteStreamLineModel(line []byte, model string) []byte {
	lineText := string(line)
	if !strings.HasPrefix(lineText, "data: ") {
		return line
	}

	data := strings.TrimSpace(strings.TrimPrefix(lineText, "data: "))
	if data == "" || data == "[DONE]" {
		return line
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		return line
	}

	payload["model"] = model
	modifiedData, err := json.Marshal(payload)
	if err != nil {
		return line
	}

	return []byte("data: " + string(modifiedData) + "\n\n")
}

func copyHeaders(dst, src http.Header) {
	skipHeaders := map[string]bool{
		"Content-Length":    true,
		"Content-Encoding":  true,
		"Transfer-Encoding": true,
		"Connection":        true,
		"Host":              true,
	}

	for key, values := range src {
		if skipHeaders[key] {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func copyResponseHeaders(dst, src http.Header) {
	skipHeaders := map[string]bool{
		"Content-Length":    true,
		"Transfer-Encoding": true,
		"Connection":        true,
	}

	for key, values := range src {
		if skipHeaders[key] {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}
