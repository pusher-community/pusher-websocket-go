pusher-websocket-go
===================

Go client (WebSocket) library for Pusher.

Status: Not currently recommended for production use. The code connects, receives messages, and reconnects on failure, but there are no tests, interfaces may change, and there are a few missing features (see todo list below).

## Usage

Create a new client. It establishes a WebSocket connection and handles automatic reconnection.

	pusher := pusher.New("<key>")

Subscribe to one or more Pusher channels. There is no need to wait for the client to connect before subscribing.

	pusher.Subscribe("<channel>")

Create a go channel on which to receive events and bind this to a given channel and event name combination. You must be subscribed to the given channel in order to receive events.

	channelEvents := make(chan string)
	pusher.OnChannelEventMessage("<channel>", "<event-name>", channelEvents)

Each time an event is received on the channel with a matching event name, the event data will be sent on the channel.

	for {
		data := <-channelEvents
		// Use data
	}

## TODO

* Read close code, adjust reconnect behaviour
* Allow disconnecting the client
* Expose client connection state changes
* More ways to bind to events and channels
* Client events
* Private channels
* Presence channels
