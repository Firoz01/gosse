package gosse_test

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/Firoz01/gosse"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSSEHandler_AddClientAndRemoveClient(t *testing.T) {
	server := gosse.NewServer()

	// Start the server
	go server.Run()
	defer server.Shutdown()

	client := server.AddClient(15)

	// Wait briefly to ensure client addition is processed
	time.Sleep(50 * time.Millisecond)

	// Verify client count
	if server.ClientCount() != 1 {
		t.Errorf("Expected client count 1, got %d", server.ClientCount())
	}

	// Remove the client
	server.RemoveClient(client.ID)

	// Wait briefly to ensure client removal is processed
	time.Sleep(50 * time.Millisecond)

	// Verify client count after removal
	if server.ClientCount() != 0 {
		t.Errorf("Expected client count 0 after removal, got %d", server.ClientCount())
	}
}

func TestSSEHandler_BroadcastMessage(t *testing.T) {
	server := gosse.NewServer()

	// Start the server
	go server.Run()
	defer server.Shutdown()

	// Add a client
	client := server.AddClient()

	// Wait briefly to ensure client addition is processed
	time.Sleep(50 * time.Millisecond)

	// Broadcast a message
	message := []byte("Test message")
	err := server.BroadcastMessage(message)
	if err != nil {
		t.Errorf("Error broadcasting message: %v", err)
	}

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
	server := gosse.NewServer()

	// Start the server
	go server.Run()
	defer server.Shutdown()

	// Create a new HTTP test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gosse.SSEHandlerEndpoint(server, w, r)
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
	err := server.BroadcastMessage(message)

	if err != nil {
		t.Errorf("Error broadcasting message: %v", err)
	}

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

func TestSSEHandler_SendMessageToClient(t *testing.T) {
	server := gosse.NewServer()

	// Start the server
	go server.Run()
	defer server.Shutdown()

	// Add a client
	client := server.AddClient()

	// Wait briefly to ensure client addition is processed
	time.Sleep(50 * time.Millisecond)

	// Send a message to the specific client
	message := []byte("Test message to specific client")
	err := server.SendMessageToClient(client.ID, message)
	if err != nil {
		t.Errorf("Error broadcasting message to client: %v", err)
	}

	// Verify the specific client received the message
	select {
	case msg := <-client.Message:
		if !bytes.Equal(msg, message) {
			t.Errorf("Expected message %s, got %s", message, msg)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for message")
	}

	// Test sending a message to a non-existent client
	nonExistentClientID := "non-existent-client-id"

	// Use recover to catch any panics caused by SendMessageToClient
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("SendMessageToClient panicked: %v", r)
		}
	}()

	// Attempt to send a message to a non-existent client
	err = server.SendMessageToClient(nonExistentClientID, message)

	// Check if the error matches the expected "not found" message
	if err != nil && err.Error() != fmt.Sprintf("client %s not found", nonExistentClientID) {
		t.Errorf("Unexpected error sending message to non-existent client: %v", err)
	}
}
