package gosse

import (
	"net/http"
)

func SSEHandlerEndpoint(handler *SSEHandler, w http.ResponseWriter, r *http.Request) {
	client := &SSEClient{
		ID:      r.RemoteAddr,
		Message: make(chan []byte),
	}

	handler.AddClient(client)
	defer handler.RemoveClient(client.ID)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	for {
		select {
		case msg, ok := <-client.Message:
			if !ok {
				return
			}
			_, err := w.Write([]byte("data: " + string(msg) + "\n\n"))
			if err != nil {
				return
			}

			flusher.Flush()

		case <-r.Context().Done():

			return
		}
	}
}
