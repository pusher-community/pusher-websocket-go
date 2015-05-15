package pusher

import (
	"encoding/json"
)

type rawMembers struct {
	Presence rawPresence `json:"presence"`
}

type rawPresence struct {
	Count int                          `json:"count"`
	Ids   []string                     `json:"ids"`
	Hash  map[string]map[string]string `json:"hash"`
}

type Members struct {
	Count   int
	Members []Member
	Me      Member
}

type Member struct {
	UserId   string            `json:"user_id"`
	UserInfo map[string]string `json:"user_info,omitempty"`
}

func unmarshalledMembers(data string, myID string) (members *Members, err error) {

	var rawData rawMembers

	err = json.Unmarshal([]byte(data), &rawData)

	var _members []Member
	var me Member

	for _, id := range rawData.Presence.Ids {
		_member := Member{UserId: id, UserInfo: rawData.Presence.Hash[id]}
		_members = append(_members, _member)

		if id == myID {
			me = _member
		}

	}

	members = &Members{
		Count:   rawData.Presence.Count,
		Members: _members,
		Me:      me,
	}

	return
}
