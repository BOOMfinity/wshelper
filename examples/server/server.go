package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/unxcepted/websocket"

	"github.com/BOOMfinity/wshelper"
	"github.com/BOOMfinity/wshelper/examples/types"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		conn, err := wshelper.Accept(w, r, nil)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("Something went wrong while processing your request"))
			return
		}
		conn.OnClose(types.OnCloseHandler)
		conn.OnError(types.OnErrorHandler)
		conn.OnMessage(func(c *wshelper.Connection, mtype websocket.MessageType, data wshelper.Payload) {
			var p types.Message
			err := data.Into(&p)
			if err != nil {
				log.Printf("Something went wrong while processing a message from a client: %v\n", err.Error())
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
				log.Println("I received a message from the client. Connection UUID: ", c.UUID())
				log.Println(hello.Message)
				_ = c.WriteJSON(context.Background(), types.SendData{
					Op: 1,
					Data: types.Hello{
						Message: fmt.Sprintf("Hey client!"),
					},
				})
				break
			}
		})
		return
	})
	log.Println("Listening on :5555")
	log.Fatalln(http.ListenAndServe(":5555", mux))
}
