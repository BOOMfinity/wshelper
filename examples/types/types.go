package types

import (
	"encoding/json"
	"log"

	"nhooyr.io/websocket"

	"github.com/BOOMfinity-Developers/wshelper"
)

var (
	OnErrorHandler = func(c *wshelper.Connection, err error) {
		log.Fatal(err)
	}
	OnCloseHandler = func(c *wshelper.Connection, code websocket.StatusCode, reason string) {
		log.Printf("The connection '%v' has been closed with code '%v' and reason '%v'", c.UUID(), code, reason)
	}
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
