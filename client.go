// Package pusher provides a client library for Pusher. It connects to the WebSocket
// interface, allows subscribing to channels, and receiving events.
package pusher

import (
	"encoding/json"
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

	*connection

	// Internal channels
	_subscribe   chan *Channel
	_unsubscribe chan string
	_disconnect  chan bool
	Connected    bool
	Channels     []*Channel
	UserData     Member
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

type evBind map[string]chan (interface{})
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
		Channels:     make([]*Channel, 0),
	}
	go client.runLoop()
	return client
}

func (self *Client) Disconnect() {
	self._disconnect <- true
}

// Subscribe subscribes the client to the channel
func (self *Client) Subscribe(channel string) (ch *Channel) {
	for _, ch := range self.Channels {
		if ch.Name == channel {
			self._subscribe <- ch
			return ch
		}
	}
	ch = &Channel{Name: channel, bindings: &self.bindings}
	self._subscribe <- ch
	return
}

// UnSubscribe unsubscribes the client from the channel
func (self *Client) Unsubscribe(channel string) {
	self._unsubscribe <- channel
}

func (self *Client) runLoop() {

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
				self.connection = c
			}

		case c := <-self._subscribe:

			if self.Connected {
				self.subscribe(c)
			}

			self.Channels = append(self.Channels, c)

		case c := <-self._unsubscribe:
			for _, ch := range self.Channels {
				if ch.Name == c {
					if self.connection != nil {
						self.unsubscribe(ch)
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
				self.connection.socketID = connectionEstablishedData["socket_id"]
				self.Connected = true
				for _, ch := range self.Channels {
					if !ch.Subscribed {
						self.subscribe(ch)
					}
				}

			case "pusher_internal:subscription_succeeded":
				for _, ch := range self.Channels {
					if ch.Name == event.Channel {
						ch.Subscribed = true
						ch.connection = self.connection
						if ch.isPresence() {
							members, _ := unmarshalledMembers(event.Data, self.UserData.UserId)
							self.triggerEventCallback(event.Channel, "pusher:subscription_succeeded", members)
						}

					}
				}

			case "pusher_internal:member_added":
				member, _ := unmarshalledMember(event.Data)
				self.triggerEventCallback(event.Channel, "pusher:member_added", member)
			case "pusher_internal:member_removed":
				member, _ := unmarshalledMember(event.Data)
				self.triggerEventCallback(event.Channel, "pusher:member_removed", member)
			default:
				self.triggerEventCallback(event.Channel, event.Name, event.Data)
			}

		case <-onClose:
			log.Print("Connection closed, will reconnect in 1s")
			for _, ch := range self.Channels {
				ch.Subscribed = false
			}
			self.connection = nil
			connectTimer.Reset(1 * time.Second)

		}
	}
}

func (self *Client) triggerEventCallback(channel, event string, data interface{}) {
	if self.bindings[channel] != nil {
		if binding := self.bindings[channel][event]; binding != nil {
			binding <- data
		}
	}
}

func encode(event string, data interface{}, channel *string) (message []byte, err error) {

	payload := map[string]interface{}{
		"event": event,
		"data":  data,
	}

	if channel != nil {
		payload["channel"] = channel
	}

	message, err = json.Marshal(payload)
	return
}

func decode(message []byte) (event Event, err error) {
	err = json.Unmarshal(message, &event)
	return
}

func (self *Client) subscribe(channel *Channel) {
	payload := map[string]string{
		"channel": channel.Name,
	}

	isPrivate := channel.isPrivate()
	isPresence := channel.isPresence()

	if isPrivate || isPresence {
		stringToSign := (s.Join([]string{self.connection.socketID, channel.Name}, ":"))
		if isPresence {
			var _userData []byte
			_userData, err := json.Marshal(self.UserData)
			if err != nil {
				panic(err)
			}
			userData := string(_userData)
			payload["channel_data"] = userData
			stringToSign = s.Join([]string{stringToSign, userData}, ":")
		}
		authString := createAuthString(self.Key, self.ClientConfig.Secret, stringToSign)
		payload["auth"] = authString
	}

	message, _ := encode("pusher:subscribe", payload, nil)
	self.connection.send(message)
}

func (self *Client) unsubscribe(channel *Channel) {
	message, _ := encode("pusher:unsubscribe", map[string]string{
		"channel": channel.Name,
	}, nil)
	self.connection.send(message)
	channel.Subscribed = false
}
