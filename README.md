# gosse Package

A simple and efficient Server-Sent Events (SSE) handler for Go using the standard library.

## Installation

```
go get github.com/Firoz01/gosse
```
## Basic Setup

``` go

func main() {
	SSEHandler := gosse.NewServer()
	go SSEHandler.Run()
	defer SSEHandler.Shutdown()

	http.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		gosse.SSEHandlerEndpoint(SSEHandler, w, r)
	})

	http.ListenAndServe(":8080", nil)
}
```

## Publishing Events

``` go
func main() {
	SSEHandler := gosse.NewServer()
	go SSEHandler.Run()
	defer SSEHandler.Shutdown()

	http.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		gosse.SSEHandlerEndpoint(SSEHandler, w, r)
	})

	http.HandleFunc("/publish", func(w http.ResponseWriter, r *http.Request) {
		eventMessage := []byte("This is a test event message")
		SSEHandler.BroadcastMessage(eventMessage)
		w.WriteHeader(http.StatusOK)
	})

	go func() {
		for {
			time.Sleep(5 * time.Second)
			eventMessage := []byte("Periodic event message")
			SSEHandler.BroadcastMessage(eventMessage)
		}
	}()

	http.ListenAndServe(":8080", nil)
}

```


## Running Tests

```sh
 go test -v 

```

