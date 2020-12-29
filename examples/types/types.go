package types

import (
	"encoding/json"
)

type Message struct {
	Op   uint8           `json:"op"`
	Data json.RawMessage `json:"data"`
}

type SendData struct {
	Op   uint8       `json:"op"`
	Data interface{} `json:"data"`
}

func (p Message) DataTo(v interface{}) error {
	return json.Unmarshal(p.Data, v)
}

type Hello struct {
	Message string `json:"message"`
}
