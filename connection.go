package pusher

import (
	// "fmt"
	"github.com/gorilla/websocket"
	"log"
	"net/url"
	"time"
)

const (
	pusherProtocol = "7"
	clientName     = "pusher-websocket-go"
	clientVersion  = "0.0.1"

	// Same as defined in websocket
	writeWait = time.Second

	// Send pongs after this time. The server inactivity timeout is 120s.
	defaultInactivityTimeout = 100 * time.Second

	// Wait this long for pong replies before closing the connection
	pongTimeout = 5000 * time.Millisecond
)

type connCallbacks struct {
	onMessage    chan<- string
	onClose      chan<- bool
	onDisconnect chan bool
}

// Connection responsibilities:
//
// * Providing a channel based interface on top of the WebSocket library
// * Connecting to the Pusher WebSocket interface
// * Triggering pings on periods of inactivity, and disconnecting if server does not reply
// * Exposing disconnect reason
//
type connection struct {
	config *connCallbacks

	inactivityTimeout time.Duration

	_sendMessage chan []byte
	_onMessage   chan string
	_onPingPong  chan bool
	_onClose     chan error
	ws           *websocket.Conn
	socketID     string
	connected    bool
}

func dial(c ClientConfig, conf *connCallbacks) (conn *connection, err error) {
	baseURL := c.Scheme + "://" + c.Host + ":" + c.Port + "/app/" + c.Key

	params := url.Values{}
	params.Set("protocol", pusherProtocol)
	params.Set("client", clientName)
	params.Set("version", clientVersion)

	url := baseURL + "?" + params.Encode()

	ws, _, err := websocket.DefaultDialer.Dial(url, nil)

	conn = &connection{
		inactivityTimeout: defaultInactivityTimeout,
		config:            conf,
		_sendMessage:      make(chan []byte, 10),
		_onMessage:        make(chan string),
		_onPingPong:       make(chan bool),
		_onClose:          make(chan error),
		ws:                ws,
	}

	// TODO: Is this blocking as it connects?

	if err == nil {
		ws.SetPingHandler(func(msg string) error {
			// TODO: Check that this is safe
			ws.WriteControl(websocket.PongMessage, []byte(msg), time.Now().Add(writeWait))
			conn._onPingPong <- true
			return nil
		})

		ws.SetPongHandler(func(msg string) error {
			conn._onPingPong <- true
			return nil
		})

		go conn.readLoop()
		go conn.runLoop()
	}

	return
}

func (self *connection) send(message []byte) {
	self._sendMessage <- message
}

func (self *connection) readLoop() {
	ws := self.ws
	for {

		if _, msg, err := ws.ReadMessage(); err == nil {
			self._onMessage <- string(msg)
		} else {
			// TODO: Read the close code

			if err.Error() == "EOF" {
				log.Print("Disconnected")
			} else {
				log.Print("Closed: ", err)
				self._onClose <- err
			}

			return
		}

	}
}

func (self *connection) runLoop() {
	pingTimer := time.NewTimer(self.inactivityTimeout)
	awaitingPong := false

	afterActivity := func() {
		pingTimer.Reset(self.inactivityTimeout)
		awaitingPong = false
	}

	ws := self.ws

	for {
		select {
		case <-pingTimer.C:
			if awaitingPong == false {
				log.Printf("No activity in %v, sending ping", self.inactivityTimeout)
				ws.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait))

				// Wait a further pong timeout
				pingTimer.Reset(pongTimeout)
				awaitingPong = true
			} else {
				log.Print("Closing after non-receipt of pong")
				ws.Close()
			}

		case <-self._onClose:
			if self.config.onClose != nil {
				self.config.onClose <- true
			}
			return

		case <-self.config.onDisconnect:
			ws.WriteControl(websocket.CloseMessage, nil, time.Now().Add(writeWait))
			log.Print("Disconnecting...")
			return

		case msg := <-self._onMessage:
			afterActivity()

			if self.config.onMessage != nil {
				self.config.onMessage <- msg
			}
		case <-self._onPingPong:
			afterActivity()

		case msg := <-self._sendMessage:

			log.Print("Sending: ", string(msg))
			err := ws.WriteMessage(websocket.TextMessage, msg)

			if err != nil {
				log.Print("Error sending: ", err)
			}
		}
	}
}
