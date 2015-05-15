package pusher

import (
	s "strings"
)

type Channel struct {
	Subscribed bool
	Name       string
	*connection
}

func (self *Channel) isPrivate() bool {
	return s.HasPrefix(self.Name, "private-")
}

func (self *Channel) isPresence() bool {
	return s.HasPrefix(self.Name, "presence-")
}

func (self *Channel) Trigger(event string, data interface{}) {
	payload, err := encode(event, data, &self.Name)

	log.Println(string(payload))

	if err != nil {
		panic(err)
	}

	self.connection.send(payload)
}
