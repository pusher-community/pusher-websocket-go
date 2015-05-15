// Package pusher provides a client library for Pusher. It connects to the WebSocket
// interface, allows subscribing to channels, and receiving events.
package pusher

import (
	"encoding/json"
	// "fmt"
	"log"
	s "strings"
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
	_subscribe   chan *Channel
	_unsubscribe chan string
	_disconnect  chan bool
	_connected   chan bool
	Connected    bool
	Channels     []*Channel
}

type ClientConfig struct {
	Scheme       string
	Host         string
	Port         string
	Key          string
	Secret       string
	AuthEndpoint string
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
		_subscribe:   make(chan *Channel),
		_unsubscribe: make(chan string),
		_disconnect:  make(chan bool),
		_connected:   make(chan bool),
		Channels:     make([]*Channel, 0),
	}
	go client.runLoop()
	return client
}

func (self *Client) Disconnect() {
	self._disconnect <- true
}

// Subscribe subscribes the client to the channel
func (self *Client) Subscribe(channel string) {
	for _, ch := range self.Channels {
		if ch.Name == channel {
			self._subscribe <- ch
			return
		}
	}
	self._subscribe <- &Channel{Name: channel}
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
	// channels := make([]Channel)

	var connection *connection

	onMessage := make(chan string)
	onClose := make(chan bool)
	onDisconnect := make(chan bool)
	callbacks := &connCallbacks{
		onMessage:    onMessage,
		onClose:      onClose,
		onDisconnect: onDisconnect,
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

				// for _, channel := range self.Channels {
				// 	self.subscribe(connection, channel)
				// }

			}

		case c := <-self._subscribe:

			if self.Connected {
				self.subscribe(connection, c)
			}

			self.Channels = append(self.Channels, c)

		case c := <-self._unsubscribe:
			for _, ch := range self.Channels {
				if ch.Name == c {
					ch.Subscribed = false
					if connection != nil {
						self.unsubscribe(connection, ch)
					}
				}
			}

		case <-self._disconnect:
			onDisconnect <- true

		case message := <-onMessage:
			event, _ := decode([]byte(message))
			log.Printf("Received: channel=%v event=%v data=%v", event.Channel, event.Name, event.Data)

			switch event.Name {
			case "pusher:connection_established":
				connectionEstablishedData := make(map[string]string)
				json.Unmarshal([]byte(event.Data), &connectionEstablishedData)
				log.Printf("%+v\n", connectionEstablishedData)
				connection.socketID = connectionEstablishedData["socket_id"]
				self.Connected = true
				for _, ch := range self.Channels {
					if !ch.Subscribed {
						self.subscribe(connection, ch)
					}
				}
			}

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

func isPrivateChannel(name string) bool {
	return s.HasPrefix(name, "private-")
}

func (self *Client) subscribe(conn *connection, channel *Channel) {
	log.Println(channel.Name)
	payload := map[string]string{
		"channel": channel.Name,
	}

	if isPrivateChannel(channel.Name) {
		stringToSign := s.Join([]string{conn.socketID, channel.Name}, ":")
		log.Printf("stringToSign: %s", stringToSign)
		authString := createAuthString(self.Key, self.ClientConfig.Secret, stringToSign)
		payload["auth"] = authString
	}

	log.Printf("%+v\n", payload)

	message, _ := encode("pusher:subscribe", payload)
	conn.send(message)
	channel.Subscribed = true
}

func (self *Client) unsubscribe(conn *connection, channel *Channel) {
	message, _ := encode("pusher:unsubscribe", map[string]string{
		"channel": channel.Name,
	})
	conn.send(message)
}
