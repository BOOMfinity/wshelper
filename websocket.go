// Simple tools for nhooyr websockets with intuitive API (in golang)
package wshelper

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/segmentio/ksuid"
	"github.com/unxcepted/websocket"
	"io"
	"net/http"
	"reflect"
	"strings"
)

var (
	Accept = func(w http.ResponseWriter, r *http.Request, opts *websocket.AcceptOptions) (*Connection, error) {
		conn, err := websocket.Accept(w, r, opts)
		if err != nil {
			return nil, err
		}
		return NewConnection(conn), nil
	}
	Dial = func(ctx context.Context, u string, opts *websocket.DialOptions) (*Connection, error) {
		conn, _, err := websocket.Dial(ctx, u, opts)
		if err != nil {
			return nil, err
		}
		return NewConnection(conn), nil
	}
)

var (
	EmptyCloseHandler = CloseHandler(func(connection *Connection, code websocket.StatusCode, reason string) {})
	EmptyErrorHandler = ErrorHandler(func(connection *Connection, err error) {})
)

type CloseHandler func(conn *Connection, code websocket.StatusCode, reason string)
type ErrorHandler func(conn *Connection, err error)
type MessageHandler func(conn *Connection, mtype websocket.MessageType, data Payload)
type MessageBufferHandler func(conn *Connection, mtype websocket.MessageType, data *bytes.Buffer)
type MessageReaderHandler func(conn *Connection, mtype websocket.MessageType, data io.Reader)

type Payload []byte

func (p Payload) Into(v interface{}) error {
	if reflect.ValueOf(v).Kind() != reflect.Ptr {
		return errors.New("A destination must be a pointer")
	}
	return json.Unmarshal(p, v)
}

type Connection struct {
	onClose CloseHandler
	onError ErrorHandler
	ws      *websocket.Conn
	uuid    string
	closed  bool
	handler interface{}
	rawData *bytes.Buffer
}

func (c *Connection) UUID() string {
	return c.uuid
}

func (c *Connection) Close(status websocket.StatusCode, reason string) error {
	if c.closed == true {
		return nil
	}
	if err := c.WS().Close(status, reason); err != nil {
		return err
	}
	c.closed = true
	return nil
}

func (c *Connection) WS() *websocket.Conn {
	return c.ws
}

func (c *Connection) WriteJSON(ctx context.Context, v interface{}) error {
	w, err := c.ws.Writer(ctx, websocket.MessageText)
	if err != nil {
		return err
	}

	// instead of json.Marshal it will reuse buffers, as we write directly to Writer
	err = json.NewEncoder(w).Encode(v)
	if err != nil {
		return err
	}

	return w.Close()
}

func (c *Connection) Write(ctx context.Context, typ websocket.MessageType, data []byte) error {
	return c.ws.Write(ctx, typ, data)
}

func (c *Connection) OnClose(h CloseHandler) {
	c.onClose = h
}

func (c *Connection) OnError(h ErrorHandler) {
	c.onError = h
}

func (c *Connection) OnMessage(h MessageHandler) {
	c.handler = h
}

func (c *Connection) OnMessageBuffer(h MessageBufferHandler) {
	c.handler = h
}

func (c *Connection) OnMessageReader(h MessageReaderHandler) {
	c.handler = h
}

func (c *Connection) loop() {
	for {
		c.rawData.Reset()
		if c.closed {
			return
		}
		t, data, err := c.ws.Reader(context.Background())
		if err != nil {
			var closeErr websocket.CloseError
			if errors.As(err, &closeErr) && c.onClose != nil {
				c.onClose(c, closeErr.Code, closeErr.Reason)
				c.closed = true
				break
			}
			if strings.Contains(err.Error(), "failed to read frame header: EOF") {
				if c.onClose != nil {
					c.onClose(c, websocket.StatusAbnormalClosure, "the connection was suddenly closed")
				}
				c.closed = true
				break
			}
			if c.onError != nil {
				c.onError(c, err)
			}
			_ = c.Close(websocket.StatusAbnormalClosure, "-")
			break
		}
		switch h := c.handler.(type) {
		case MessageHandler:
			_, err = c.rawData.ReadFrom(data)
			if err == nil {
				h(c, t, c.rawData.Bytes())
			}
		case MessageBufferHandler:
			_, err = c.rawData.ReadFrom(data)
			if err == nil {
				h(c, t, c.rawData)
			}
		case MessageReaderHandler:
			h(c, t, data)
		}
	}
}

func NewConnection(conn *websocket.Conn) *Connection {
	c := &Connection{
		ws:      conn,
		uuid:    ksuid.New().String(),
		onClose: nil,
		onError: nil,
		handler: nil,
		rawData: new(bytes.Buffer),
	}
	go c.loop()
	return c
}
