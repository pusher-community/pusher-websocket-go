package pusher

import (
	s "strings"
)

type Channel struct {
	Subscribed bool
	Name       string
}

func (self *Channel) isPrivate() bool {
	return s.HasPrefix(self.Name, "private-")
}

func (self *Channel) isPresence() bool {
	return s.HasPrefix(self.Name, "presence-")
}
