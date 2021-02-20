package main

import (
	"context"
	"log"

	"nhooyr.io/websocket"

	"github.com/BOOMfinity-Developers/wshelper"
	"github.com/BOOMfinity-Developers/wshelper/examples/types"
)

func main() {
	conn, err := wshelper.Dial(context.Background(), "ws://localhost:5555", nil)
	if err != nil {
		log.Fatal("Something went wrong while connecting to the server: ", err.Error())
	}
	conn.OnClose(types.OnCloseHandler)
	conn.OnError(types.OnErrorHandler)
	conn.OnMessage(func(c *wshelper.Connection, mtype websocket.MessageType, data wshelper.Payload) {
		var p types.Message
		err := data.Into(&p)
		if err != nil {
			log.Printf("Something went wrong while processing a message from a server: %v\n", err.Error())
			return
		}
		switch p.Op {
		case 1:
			var hello types.Hello
			err = p.DataTo(&hello)
			if err != nil {
				log.Printf("Something went wrong while processing the payload data: %v\n", err.Error())
				return
			}
			log.Println("I received a message from the server. Connection UUID: ", c.UUID())
			log.Println(hello.Message)
			println()
			log.Println("Closing connection")
			conn.WS().Close(websocket.StatusNormalClosure, "why not")
			break
		}
	})
	conn.WriteJSON(context.Background(), types.SendData{
		Op: 1,
		Data: types.Hello{
			Message: "Hey server!",
		},
	})
	log.Println("Connected and ready to receive messages")
	cl := make(chan bool)
	<-cl
}
