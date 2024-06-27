package gosse_test

import (
	"bufio"
	"bytes"
	"github.com/Firoz01/gosse"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSSEHandler_AddClientAndRemoveClient(t *testing.T) {
	handler := gosse.NewSSEHandler()
	clientID := "client1"

	// Add a client
	client := &gosse.SSEClient{
		ID:      clientID,
		Message: make(chan []byte, 10),
	}

	go handler.Run()

	defer func() {
		handler.Shutdown()
	}()

	handler.AddClient(client)

	// Wait briefly to ensure client addition is processed
	time.Sleep(50 * time.Millisecond)

	// Verify client count
	if handler.ClientCount() != 1 {
		t.Errorf("Expected client count 1, got %d", handler.ClientCount())
	}

	// Remove the client
	handler.RemoveClient(clientID)

	// Wait briefly to ensure client removal is processed
	time.Sleep(50 * time.Millisecond)

	// Verify client count after removal
	if handler.ClientCount() != 0 {
		t.Errorf("Expected client count 0 after removal, got %d", handler.ClientCount())
	}
}

func TestSSEHandler_BroadcastMessage(t *testing.T) {
	handler := gosse.NewSSEHandler()
	clientID := "client1"

	// Add a client
	client := &gosse.SSEClient{
		ID:      clientID,
		Message: make(chan []byte, 10),
	}

	go handler.Run()

	defer func() {
		handler.Shutdown()
	}()

	handler.AddClient(client)

	// Broadcast a message
	message := []byte("Test message")
	handler.BroadcastMessage(message)

	// Verify client received the message
	select {
	case msg := <-client.Message:
		if !bytes.Equal(msg, message) {
			t.Errorf("Expected message %s, got %s", message, msg)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for message")
	}
}

func TestSSEHandlerEndpoint(t *testing.T) {
	handler := gosse.NewSSEHandler()
	go handler.Run()
	defer handler.Shutdown()

	// Create a new HTTP test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gosse.SSEHandlerEndpoint(handler, w, r)
	}))
	defer ts.Close()

	// Channel to signal when SSE messages are received
	received := make(chan string)

	// Start a Goroutine to simulate SSE message reception
	go func() {
		// Perform a request to the test server
		req, err := http.NewRequest("GET", ts.URL, nil)
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		// Perform the request
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		// Check response status
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status OK, got %s", resp.Status)
		}

		// Validate Content-Type header
		contentType := resp.Header.Get("Content-Type")
		if contentType != "text/event-stream" {
			t.Errorf("Expected Content-Type text/event-stream, got %s", contentType)
		}

		// Use bufio.NewReader to read the response line by line
		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				t.Fatalf("Failed to read SSE response body: %v", err)
			}
			if strings.HasPrefix(line, "data: ") {
				received <- line
				return
			}
		}
	}()

	// Delay to ensure the client is connected before broadcasting
	time.Sleep(100 * time.Millisecond)
	message := []byte("Test message")
	handler.BroadcastMessage(message)

	select {
	case msg := <-received:
		expectedSSEFormat := "data: Test message\n"
		if !strings.Contains(msg, expectedSSEFormat) {
			t.Errorf("Expected SSE message format %q, got %q", expectedSSEFormat, msg)
		}
	case <-time.After(5 * time.Second):
		t.Error("Timeout waiting for SSE message")
	}
}
