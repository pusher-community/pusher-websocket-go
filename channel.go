package pusher

import (
	s "strings"
)

type Channel struct {
	Subscribed bool
	Name       string
	*connection
	bindings *chanbindings
}

type EventHandler func(data interface{})

func (self *Channel) isPrivate() bool {
	return s.HasPrefix(self.Name, "private-")
}

func (self *Channel) isPresence() bool {
	return s.HasPrefix(self.Name, "presence-")
}

func (self *Channel) Trigger(event string, data interface{}) {
	payload, err := encode(event, data, &self.Name)

	if err != nil {
		panic(err)
	}

	self.connection.send(payload)
}

func (self *Channel) Bind(event string, callback EventHandler) {
	bindings := *self.bindings
	if bindings[self.Name] == nil {
		bindings[self.Name] = make(map[string]chan (interface{}))
	}

	channelEvents := make(chan interface{})

	bindings[self.Name][event] = channelEvents

	go func() {
		for {
			data := <-channelEvents
			callback(data)
		}
	}()

}
