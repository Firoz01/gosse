// SSE (Server-Sent Events) is a technology that allows a server to push real-time updates to a client
// over a single, long-lived HTTP connection. It is a simple and efficient way to stream updates from
// the server to the client without requiring the client to repeatedly poll the server for new data.

package gosse

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"
)

// Client represents a single SSE (Server-Sent Events) client connection.
// It contains fields for uniquely identifying the client, managing message
// communication, and tracking connection and activity times.
type Client struct {
	ID           string      // Unique identifier for the client.
	Message      chan []byte // Channel for receiving messages from the server.
	ConnectedAt  time.Time   // Timestamp when the client initially connected to the server.
	LastActiveAt time.Time   // Timestamp of the client's last activity, updated on each message received.
}

// Server manages the connected SSE (Server-Sent Events) clients.
// It uses a thread-safe sync.Map to store clients, and channels (add and remove)
// for adding and removing clients respectively. The done channel signals
// shutdown, and clientCount tracks the current number of connected clients
// with clientCountM used to synchronize updates safely.
type Server struct {
	clients      sync.Map      // Map to store connected clients (thread-safe)
	add          chan *Client  // Channel for adding clients
	remove       chan string   // Channel for removing clients by ID
	done         chan struct{} // Channel to signal shutdown
	clientCount  int           // Track current number of clients
	clientCountM sync.Mutex    // Mutex to synchronize client count updates
}

// NewServer creates a new Server instance with initialized fields.
// It sets up a thread-safe map for storing connected clients,
// channels for adding and removing clients, and signaling shutdown.
// The client count is initialized to zero, and a mutex is used to synchronize
// updates to the client count.
func NewServer() *Server {
	return &Server{
		clients:      sync.Map{},          // Initialize thread-safe map for clients
		add:          make(chan *Client),  // Initialize channel for adding clients
		remove:       make(chan string),   // Initialize channel for removing clients
		done:         make(chan struct{}), // Initialize channel for signaling shutdown
		clientCount:  0,                   // Initialize client count
		clientCountM: sync.Mutex{},        // Initialize mutex for client count synchronization
	}
}

// Run starts the Server to manage SSE clients asynchronously.
// It listens for operations on the 'add', 'remove', and 'done' channels:
//
//   - 'add': Adds a new client to the server's client map and increments the client count.
//     The new client is stored in the sync.Map with its generated ID.
//   - 'remove': Removes a client from the server by its ID. The client is deleted from the client map,
//     its message channel is closed, and the client count is decremented.
//   - 'done': Signals shutdown. The server cleans up all clients by closing their message channels,
//     ensuring a graceful shutdown.
//
// This function runs indefinitely until the 'done' channel is closed,
// ensuring proper client management and shutdown handling in a concurrent environment.
func (s *Server) Run() {
	for {
		select {
		case client := <-s.add:
			// Add client to the map with generated ID
			s.clients.Store(client.ID, client)
			// Increment client count safely
			s.incrementClientCount()

		case clientID := <-s.remove:
			// Remove client from the map by ID
			if client, ok := s.clients.Load(clientID); ok {
				s.clients.Delete(clientID)
				// Close client's message channel
				close(client.(*Client).Message)
				// Decrement client count safely
				s.decrementClientCount()
			}

		case <-s.done:
			// Cleanup all clients on shutdown
			s.clients.Range(func(key, value interface{}) bool {
				client := value.(*Client)
				close(client.Message) // Close client's message channel
				return true
			})
			return
		}
	}
}

// AddClient adds a new client to the server.
// It optionally accepts a buffer size for the client's message channel.
// If no buffer size is specified, a default size of 10 is used.
//
// Parameters:
//   - bufferSize: Optional integer specifying the size of the buffered channel for messages.
//
// Returns:
//   - *Client: A pointer to the newly created Client instance.
func (s *Server) AddClient(bufferSize ...int) *Client {
	size := 10 // Default buffer size
	if len(bufferSize) > 0 {
		size = bufferSize[0] // Use the provided buffer size if specified
	}
	client := &Client{
		ID:           s.generateClientID(),
		Message:      make(chan []byte, size), // Use the specified or default buffer size
		ConnectedAt:  time.Now(),
		LastActiveAt: time.Now(),
	}
	s.add <- client // Send client to 'add' channel for processing in Run()
	return client
}

// RemoveClient removes a client from the server by ID.
// It sends the client ID to the 'remove' channel for processing in the Run method.
//
// Parameters:
//   - clientID: The unique identifier of the client to be removed.
func (s *Server) RemoveClient(clientID string) {
	s.remove <- clientID // Send clientID to 'remove' channel for processing in Run()
}

// BroadcastMessage sends a message to all connected clients.
// It iterates over the clients stored in the Server's sync.Map (`clients`), attempting
// to send the provided `msg` to each client's Message channel. This is done in a non-blocking
// manner to ensure the server continues functioning even if some clients are not ready to receive messages.
//
// The function does the following:
// 1. Retrieves each client from the sync.Map (`clients`).
// 2. Attempts to send the provided message (`msg`) to the client's Message channel.
// 3. Updates the client's LastActiveAt timestamp to the current time if the message is successfully sent.
// 4. Logs a message indicating the client's unavailability if the Message channel is not ready to receive the message.
//
// Parameters:
//   - msg: The message to be sent to all connected clients, represented as a byte slice.
func (s *Server) BroadcastMessage(msg []byte) error {
	var err error
	s.clients.Range(func(key, value interface{}) bool {
		client := value.(*Client)
		select {
		case client.Message <- msg:
			client.LastActiveAt = time.Now()
		default:
			err = fmt.Errorf("client %s is not ready to receive messages", key)
		}
		return true
	})
	return err
}

// SendMessageToClient sends a message to a specific client by their ID.
// It retrieves the client's connection from the server's sync.Map (`clients`)
// and attempts to send the provided `msg` to the client's Message channel.
// If the client is not found, or if the client's Message channel is not ready to
// receive the message (non-blocking send), it returns an appropriate error.
func (s *Server) SendMessageToClient(clientID string, msg []byte) error {
	if client, ok := s.clients.Load(clientID); ok {
		select {
		case client.(*Client).Message <- msg: // Send message to client's message channel
			client.(*Client).LastActiveAt = time.Now()
			return nil
		default:
			return fmt.Errorf("client %s is not ready to receive messages", clientID)
		}
	} else {
		return fmt.Errorf("client %s not found", clientID)
	}
}

// Shutdown gracefully shuts down the SSE server.
// It closes the 'done' channel, which signals the Run() method to initiate
// shutdown and cleanup of all connected clients.
func (s *Server) Shutdown() {
	close(s.done) // Signal 'done' channel to initiate shutdown in Run()
}

// ClientCount returns the current number of connected clients.
// It synchronizes access to the client count using a mutex to prevent
// concurrent modifications during read operations.
func (s *Server) ClientCount() int {
	s.clientCountM.Lock()
	defer s.clientCountM.Unlock()
	return s.clientCount // Return current client count
}

// incrementClientCount safely increments the client count.
// It acquires a mutex lock before incrementing to prevent
// concurrent modifications to the client count.
func (s *Server) incrementClientCount() {
	s.clientCountM.Lock()
	defer s.clientCountM.Unlock()
	s.clientCount++ // Increment client count
}

// decrementClientCount safely decrements the client count.
// It acquires a mutex lock before decrementing to prevent
// concurrent modifications to the client count.
func (s *Server) decrementClientCount() {
	s.clientCountM.Lock()
	defer s.clientCountM.Unlock()
	s.clientCount-- // Decrement client count
}

// generateClientID generates a unique identifier for a client.
// The identifier is a 20-character long base64 URL-safe string derived from random bytes.
// The function ensures the uniqueness of the generated ID within the server's client map.
//
// The function performs the following steps:
// 1. Defines a constant `idLength` which sets the length of the client ID to 20 characters.
// 2. Generates a random byte slice of length `idLength`.
// 3. Encodes the byte slice to a base64 URL-safe string and truncates it to `idLength`.
// 4. Checks if the generated ID is unique within the server's client map:
//   - If the ID is unique, it is stored in the client map and returned.
//   - If the ID already exists, a new random byte slice is generated, and the process is repeated.
//
// Error Handling:
// - If the random byte generation fails, the function panics with the encountered error.
//
// Returns:
// - A unique client ID as a 20-character long base64 URL-safe string.
func (s *Server) generateClientID() string {
	const idLength = 20 // Length of the client ID

	// Generate a random byte slice of appropriate length
	randomBytes := make([]byte, idLength)
	_, err := rand.Read(randomBytes)
	if err != nil {
		// Handle error, if any
		panic(err) // Example: for simplicity
	}

	// Encode the byte slice to a base64 URL-safe string
	clientID := base64.URLEncoding.EncodeToString(randomBytes)[:idLength]

	// Ensure the generated ID is unique
	for {
		_, exists := s.clients.LoadOrStore(clientID, struct{}{})
		if !exists {
			break
		}
		// If ID already exists, generate a new one
		_, err := rand.Read(randomBytes)
		if err != nil {
			// Handle error, if any
			panic(err) // Example: for simplicity
		}
		clientID = base64.URLEncoding.EncodeToString(randomBytes)[:idLength]
	}

	return clientID
}
