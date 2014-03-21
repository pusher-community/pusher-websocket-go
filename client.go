// Package pusher provides a client library for Pusher. It connects to the WebSocket
// interface, allows subscribing to channels, and receiving events.
package pusher

import (
	"encoding/json"
	"log"
	"time"
)

const (
	// Default WebSocket endpoint
	defaultScheme = "wss"
	defaultHost   = "ws.pusherapp.com"
	defaultPort   = "443"
)

// Client responsibilities:
//
// * Connecting (via Connection)
// * Reconnecting on disconnect
// * Decoding and encoding events
// * Managing channel subscriptions
//
type Client struct {
	ClientConfig

	bindings chanbindings

	// Internal channels
	_subscribe   chan string
	_unsubscribe chan string
}

type ClientConfig struct {
	Scheme string
	Host   string
	Port   string
	Key    string
}

type Event struct {
	Name    string `json:"event"`
	Channel string `json:"channel"`
	Data    string `json:"data"`
}

type evBind map[string]chan (string)
type chanbindings map[string]evBind

// New creates a new Pusher client with given Pusher application key
func New(key string) *Client {
	config := ClientConfig{
		Scheme: defaultScheme,
		Host:   defaultHost,
		Port:   defaultPort,
		Key:    key,
	}
	return NewWithConfig(config)
}

// NewWithConfig allows creating a new Pusher client which connects to a custom endpoint
func NewWithConfig(c ClientConfig) *Client {
	client := &Client{
		ClientConfig: c,
		bindings:     make(chanbindings),
		_subscribe:   make(chan string),
		_unsubscribe: make(chan string),
	}
	go client.runLoop()
	return client
}

// Subscribe subscribes the client to the channel
func (self *Client) Subscribe(channel string) {
	self._subscribe <- channel
}

// UnSubscribe unsubscribes the client from the channel
func (self *Client) Unsubscribe(channel string) {
	self._unsubscribe <- channel
}

func (self *Client) OnChannelEventMessage(channelName, eventName string, c chan string) {
	// Register callback function
	if self.bindings[channelName] == nil {
		self.bindings[channelName] = make(map[string]chan (string))
	}

	self.bindings[channelName][eventName] = c
}

func (self *Client) runLoop() {
	// Run loop state
	channels := make(map[string]bool)
	var connection *connection

	onMessage := make(chan string)
	onClose := make(chan bool)
	callbacks := &connCallbacks{
		onMessage: onMessage,
		onClose:   onClose,
	}

	// Connect when this timer fires - initially fire immediately
	connectTimer := time.NewTimer(0 * time.Second)

	for {
		select {
		case <-connectTimer.C:
			// Connect to Pusher
			if c, err := dial(self.ClientConfig, callbacks); err != nil {
				log.Print("Failed to connect: ", err)
				connectTimer.Reset(1 * time.Second)
			} else {
				log.Print("Connection opened")
				connection = c

				// Subscribe to all channels
				for ch, _ := range channels {
					subscribe(connection, ch)
				}
			}

		case c := <-self._subscribe:
			channels[c] = true
			if connection != nil {
				subscribe(connection, c)
			}

		case c := <-self._unsubscribe:
			delete(channels, c)
			if connection != nil {
				unsubscribe(connection, c)
			}

		case message := <-onMessage:
			event, _ := decode([]byte(message))
			log.Printf("Received: channel=%v event=%v", event.Channel, event.Name)

			if self.bindings[event.Channel] != nil {
				if self.bindings[event.Channel][event.Name] != nil {
					self.bindings[event.Channel][event.Name] <- event.Data
				}
			}

		case <-onClose:
			log.Print("Connection closed, will reconnect in 1s")
			connection = nil
			connectTimer.Reset(1 * time.Second)
		}
	}
}

func encode(event string, data interface{}) (message []byte, err error) {
	message, err = json.Marshal(map[string]interface{}{
		"event": event,
		"data":  data,
	})
	return
}

func decode(message []byte) (event Event, err error) {
	err = json.Unmarshal(message, &event)
	return
}

func subscribe(conn *connection, channel string) {
	message, _ := encode("pusher:subscribe", map[string]string{
		"channel": channel,
	})
	conn.send(message)
}

func unsubscribe(conn *connection, channel string) {
	message, _ := encode("pusher:unsubscribe", map[string]string{
		"channel": channel,
	})
	conn.send(message)
}
