// Simple tools for nhooyr websockets with intuitive API (in golang)
package wshelper

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
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

type closeHandler func(connection *Connection, code websocket.StatusCode, reason string)
type errorHandler func(connection *Connection, err error)
type messageHandler func(connection *Connection, data Payload)

type Payload []byte

func (p Payload) Into(v interface{}) error {
	if reflect.ValueOf(v).Kind() != reflect.Ptr {
		return errors.New("A destination must be a pointer")
	}
	return json.Unmarshal(p, v)
}

type handler struct {
	conn        *Connection
	id          uint64
	synchronous bool
	run         messageHandler
}

func (h *handler) SetAsync(enabled bool) {
	h.synchronous = !enabled
}

func (h handler) Delete() {
	h.conn.removeHandler(h.id)
}

type Connection struct {
	onClose   *closeHandler
	onError   *errorHandler
	ws        *websocket.Conn
	handlers  map[uint64]*handler
	handlerID *atomic.Uint64
	mutex     sync.Mutex
	uuid      string
}

func (c *Connection) UUID() string {
	return c.uuid
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

func (c *Connection) OnClose(h closeHandler) {
	c.onClose = &h
}

func (c *Connection) OnError(h errorHandler) {
	c.onError = &h
}

func (c *Connection) OnMessage(h messageHandler) *handler {
	p := new(handler)
	p.id = c.handlerID.Inc()
	p.run = h
	c.mutex.Lock()
	c.handlers[p.id] = p
	c.mutex.Unlock()
	return p
}

func (c *Connection) loop() {
	for {
		_, data, err := c.ws.Read(context.Background())
		if err != nil {
			var closeErr websocket.CloseError
			if errors.As(err, &closeErr) && c.onClose != nil {
				(*c.onClose)(c, closeErr.Code, closeErr.Reason)
				break
			}
			if c.onError != nil {
				(*c.onError)(c, err)
			}
			break
		}
		c.mutex.Lock()
		for _, h := range c.handlers {
			if h.synchronous {
				h.run(c, data)
			} else {
				go h.run(c, data)
			}
		}
		c.mutex.Unlock()
	}
}

func NewConnection(conn *websocket.Conn) *Connection {
	c := &Connection{
		ws:        conn,
		handlers:  make(map[uint64]*handler),
		handlerID: atomic.NewUint64(0),
		uuid:      ksuid.New().String(),
	}
	go c.loop()
	return c
}
