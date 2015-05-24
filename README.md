pusher-websocket-go
===================

Go client (WebSocket) library for Pusher.

Status: Not currently recommended for production use. The code connects, receives messages, and reconnects on failure, but there are no tests, interfaces may change, and there are a few missing features (see todo list below).

## Usage

Create a new client. It establishes a WebSocket connection and handles automatic reconnection.

```go
client := pusher.New("<key>")
```

Subscribe to one or more Pusher channels. There is no need to wait for the client to connect before subscribing.

```go
channel := pusher.Subscribe("<channel>")
```

To bind to events:

```go
channel.bind("my-event", func(data interface{}){
  fmt.Println(data)
})
```

## TODO

* Read close code, adjust reconnect behaviour
* Expose client connection state changes
