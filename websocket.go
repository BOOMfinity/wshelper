// Simple tools for nhooyr websockets with intuitive API (in golang)
package wshelper

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"sync"

	"github.com/segmentio/ksuid"
	"go.uber.org/atomic"
	"nhooyr.io/websocket"
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

type Payload []byte

func (p Payload) Into(v interface{}) error {
	if reflect.ValueOf(v).Kind() != reflect.Ptr {
		return errors.New("A destination must be a pointer")
	}
	return json.Unmarshal(p, v)
}

type Handler struct {
	conn        *Connection
	id          uint64
	synchronous bool
	run         interface{}
}

func (h *Handler) SetAsync(enabled bool) {
	h.synchronous = !enabled
}

func (h Handler) Delete() {
	h.conn.removeHandler(h.id)
}

func (h Handler) exec(mtype websocket.MessageType, data []byte) {
	switch run := h.run.(type) {
	case MessageHandler:
		run(h.conn, mtype, data)
		break
	case MessageBufferHandler:
		run(h.conn, mtype, bytes.NewBuffer(data))
		break
	}
}

func (h Handler) check() (ok bool) {
	switch h.run.(type) {
	case MessageHandler:
		ok = true
		break
	case MessageBufferHandler:
		ok = true
		break
	}
	return
}

type Connection struct {
	onClose   CloseHandler
	onError   ErrorHandler
	ws        *websocket.Conn
	handlers  map[uint64]*Handler
	handlerID *atomic.Uint64
	mutex     sync.Mutex
	uuid      string
	closed    bool
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

func (c *Connection) removeHandler(id uint64) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	delete(c.handlers, id)
}

func (c *Connection) OnClose(h CloseHandler) {
	c.onClose = h
}

func (c *Connection) OnError(h ErrorHandler) {
	c.onError = h
}

func (c *Connection) newHandler() *Handler {
	p := new(Handler)
	p.id = c.handlerID.Inc()
	p.conn = c
	c.mutex.Lock()
	c.handlers[p.id] = p
	c.mutex.Unlock()
	return p
}

func (c *Connection) OnMessage(h MessageHandler) *Handler {
	p := c.newHandler()
	p.run = h
	return p
}

func (c *Connection) OnMessageBuffer(h MessageBufferHandler) *Handler {
	p := c.newHandler()
	p.run = h
	return p
}

func (c *Connection) loop() {
	for {
		if c.closed {
			return
		}
		t, data, err := c.ws.Read(context.Background())
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
			continue
		}
		c.mutex.Lock()
		for _, h := range c.handlers {
			if h.synchronous {
				h.exec(t, data)
			} else {
				go h.exec(t, data)
			}
		}
		c.mutex.Unlock()
	}
}

func NewConnection(conn *websocket.Conn) *Connection {
	c := &Connection{
		ws:        conn,
		handlers:  make(map[uint64]*Handler),
		handlerID: atomic.NewUint64(0),
		uuid:      ksuid.New().String(),
		onClose:   nil,
		onError:   nil,
	}
	go c.loop()
	return c
}
