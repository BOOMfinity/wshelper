package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"nhooyr.io/websocket"

	"github.com/BOOMfinity-Developers/wshelper"
	"github.com/BOOMfinity-Developers/wshelper/examples/types"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		conn, err := wshelper.Accept(w, r, nil)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Something went wrong while processing your request"))
			return
		}
		conn.OnClose(func(c *wshelper.Connection, code websocket.StatusCode, reason string) {
			log.Printf("The connection (%v) has been closed with code %v and reason %v\n", c.UUID(), code, reason)
		})
		conn.OnError(func(c *wshelper.Connection, err error) {
			log.Fatal(err)
		})
		conn.OnMessage(func(c *wshelper.Connection, data wshelper.Payload) {
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
				c.WriteJSON(context.Background(), types.SendData{
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
	http.ListenAndServe(":5555", mux)
}
