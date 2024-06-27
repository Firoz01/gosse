// SSE (Server-Sent Events) is a technology that allows a server to push real-time updates to a client
// over a single, long-lived HTTP connection. It is a simple and efficient way to stream updates from
// the server to the client without requiring the client to repeatedly poll the server for new data.

package gosse

import (
	"fmt"
	"sync"
)

// SSEClient represents a single SSE (Server-Sent Events) client connection.
// It contains an ID for uniquely identifying the client and a Message channel
// for receiving messages from the server.
type SSEClient struct {
	ID      string      // Unique identifier for the client
	Message chan []byte // Channel for receiving messages
}

// SSEHandler manages the connected SSE (Server-Sent Events) clients.
// It uses a thread-safe sync.Map to store clients, and channels (add and remove)
// for adding and removing clients respectively. The done channel signals
// shutdown, and clientCount tracks the current number of connected clients
// with clientCountM used to synchronize updates safely.
type SSEHandler struct {
	clients      sync.Map        // Map to store connected clients (thread-safe)
	add          chan *SSEClient // Channel for adding clients
	remove       chan string     // Channel for removing clients by ID
	done         chan struct{}   // Channel to signal shutdown
	clientCount  int             // Track current number of clients
	clientCountM sync.Mutex      // Mutex to synchronize client count updates
}

// NewSSEHandler creates a new SSEHandler instance with initialized fields.
// It sets up a thread-safe map for storing connected clients,
// channels for adding and removing clients, and signaling shutdown.
// The client count is initialized to zero, and a mutex is used to synchronize
// updates to the client count.
func NewSSEHandler() *SSEHandler {
	return &SSEHandler{
		clients:      sync.Map{},            // Initialize thread-safe map for clients
		add:          make(chan *SSEClient), // Initialize channel for adding clients
		remove:       make(chan string),     // Initialize channel for removing clients
		done:         make(chan struct{}),   // Initialize channel for signaling shutdown
		clientCount:  0,                     // Initialize client count
		clientCountM: sync.Mutex{},          // Initialize mutex for client count synchronization
	}
}

// Run starts the SSEHandler to manage SSE clients asynchronously.
// It listens for operations on the 'add', 'remove', and 'done' channels:
//   - 'add': Adds a new client to the handler's client map and increments the client count.
//   - 'remove': Removes a client from the handler by ID, deletes it from the client map,
//     closes its message channel, and decrements the client count.
//   - 'done': Signals shutdown, cleans up all clients by closing their message channels.
//
// This function runs indefinitely until the 'done' channel is closed,
// ensuring proper client management and shutdown handling in a concurrent environment.
func (h *SSEHandler) Run() {
	for {
		select {
		case client := <-h.add:
			// Add client to the map
			h.clients.Store(client.ID, client)
			// Increment client count safely
			h.incrementClientCount()

		case clientID := <-h.remove:
			// Remove client from the map by ID
			if client, ok := h.clients.Load(clientID); ok {
				h.clients.Delete(clientID)
				// Close client's message channel
				close(client.(*SSEClient).Message)
				// Decrement client count safely
				h.decrementClientCount()

			}

		case <-h.done:
			// Cleanup all clients on shutdown
			h.clients.Range(func(key, value interface{}) bool {
				client := value.(*SSEClient)
				close(client.Message) // Close client's message channel
				return true
			})
			return
		}
	}
}

// AddClient adds a new client to the handler
func (h *SSEHandler) AddClient(c *SSEClient) {
	h.add <- c // Send client to 'add' channel for processing in Run()
}

// RemoveClient removes a client from the handler by ID
func (h *SSEHandler) RemoveClient(clientID string) {
	h.remove <- clientID // Send clientID to 'remove' channel for processing in Run()
}

// BroadcastMessage sends a message to all connected clients.
// It iterates over the clients stored in the SSEHandler's sync.Map (`clients`),
// attempting to send the provided `msg` to each client's Message channel.
// If a client's Message channel is not ready to receive the message (non-blocking send),
// it logs a message indicating the client's unavailability.
func (h *SSEHandler) BroadcastMessage(msg []byte) {
	h.clients.Range(func(key, value interface{}) bool {
		client := value.(*SSEClient)
		select {
		case client.Message <- msg: // Send message to client's message channel
		default:
			fmt.Printf("Client %s is not ready to receive messages\n", client.ID)
		}
		return true
	})
}

// Shutdown gracefully shuts down the SSE handler.
// It closes the 'done' channel, which signals the Run() method to initiate
// shutdown and cleanup of all connected clients.
func (h *SSEHandler) Shutdown() {
	close(h.done) // Signal 'done' channel to initiate shutdown in Run()
}

// ClientCount returns the current number of connected clients.
// It synchronizes access to the client count using a mutex to prevent
// concurrent modifications during read operations.
func (h *SSEHandler) ClientCount() int {
	h.clientCountM.Lock()
	defer h.clientCountM.Unlock()
	return h.clientCount // Return current client count
}

// incrementClientCount safely increments the client count.
// It acquires a mutex lock before incrementing to prevent
// concurrent modifications to the client count.
func (h *SSEHandler) incrementClientCount() {
	h.clientCountM.Lock()
	defer h.clientCountM.Unlock()
	h.clientCount++ // Increment client count
}

// decrementClientCount safely decrements the client count.
// It acquires a mutex lock before decrementing to prevent
// concurrent modifications to the client count.
func (h *SSEHandler) decrementClientCount() {
	h.clientCountM.Lock()
	defer h.clientCountM.Unlock()
	h.clientCount-- // Decrement client count
}
