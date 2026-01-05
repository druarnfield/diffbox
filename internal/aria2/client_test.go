package aria2

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient(t *testing.T) {
	client := NewClient("localhost", 6800, "secret123")

	if client.url != "http://localhost:6800/jsonrpc" {
		t.Errorf("expected url http://localhost:6800/jsonrpc, got %s", client.url)
	}

	if client.secret != "secret123" {
		t.Errorf("expected secret 'secret123', got %s", client.secret)
	}

	if client.httpClient == nil {
		t.Error("expected http client to be initialized")
	}
}

func TestClientGetVersion(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		if req.Method != "aria2.getVersion" {
			t.Errorf("expected method aria2.getVersion, got %s", req.Method)
		}

		response := Response{
			ID:     req.ID,
			Result: json.RawMessage(`{"version": "1.37.0"}`),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Extract host and port from server URL
	client := &Client{
		url:        server.URL,
		httpClient: server.Client(),
	}

	version, err := client.GetVersion()
	if err != nil {
		t.Fatalf("GetVersion failed: %v", err)
	}

	if version != "1.37.0" {
		t.Errorf("expected version 1.37.0, got %s", version)
	}
}

func TestClientAddURI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		if req.Method != "aria2.addUri" {
			t.Errorf("expected method aria2.addUri, got %s", req.Method)
		}

		// Check params
		if len(req.Params) < 2 {
			t.Error("expected at least 2 params")
		}

		response := Response{
			ID:     req.ID,
			Result: json.RawMessage(`"abc123"`),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := &Client{
		url:        server.URL,
		httpClient: server.Client(),
	}

	gid, err := client.AddURI("https://example.com/file.bin", "/downloads", "file.bin", nil)
	if err != nil {
		t.Fatalf("AddURI failed: %v", err)
	}

	if gid != "abc123" {
		t.Errorf("expected gid abc123, got %s", gid)
	}
}

func TestClientTellStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Request
		json.NewDecoder(r.Body).Decode(&req)

		response := Response{
			ID: req.ID,
			Result: json.RawMessage(`{
				"gid": "abc123",
				"status": "active",
				"totalLength": "1000000",
				"completedLength": "500000",
				"downloadSpeed": "100000"
			}`),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := &Client{
		url:        server.URL,
		httpClient: server.Client(),
	}

	status, err := client.TellStatus("abc123")
	if err != nil {
		t.Fatalf("TellStatus failed: %v", err)
	}

	if status.GID != "abc123" {
		t.Errorf("expected gid abc123, got %s", status.GID)
	}

	if status.Status != "active" {
		t.Errorf("expected status active, got %s", status.Status)
	}
}

func TestClientRPCError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Request
		json.NewDecoder(r.Body).Decode(&req)

		response := Response{
			ID: req.ID,
			Error: &RPCError{
				Code:    1,
				Message: "test error",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := &Client{
		url:        server.URL,
		httpClient: server.Client(),
	}

	_, err := client.GetVersion()
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestParamsAlwaysArray(t *testing.T) {
	// Test that params is always an array, never null
	// This validates the fix for aria2 RPC error -32602 "Invalid params"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read raw body to check exact JSON format
		var rawRequest map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&rawRequest); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		// Verify params exists and is an array (not null)
		params, ok := rawRequest["params"]
		if !ok {
			t.Error("params field missing from request")
		}

		// Check that params is a slice (array in JSON), not nil
		if params == nil {
			t.Error("params is null, should be empty array []")
		}

		// Verify it's actually an array type
		if _, isArray := params.([]interface{}); !isArray {
			t.Errorf("params is not an array, got type %T", params)
		}

		// Send success response
		var req Request
		json.Unmarshal([]byte(rawRequest["id"].(string)), &req.ID)
		response := Response{
			ID:     rawRequest["id"].(string),
			Result: json.RawMessage(`{"version": "1.37.0"}`),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := &Client{
		url:        server.URL,
		httpClient: server.Client(),
	}

	// Call GetVersion which takes no params - this is where the bug would occur
	_, err := client.GetVersion()
	if err != nil {
		t.Fatalf("GetVersion failed: %v", err)
	}
}
