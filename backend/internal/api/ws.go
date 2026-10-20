package api

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/Salimzyanov-Lenar/graph-war/backend/internal/rooms"
)

const (
	wsQueueSize     = 64
	wsReadLimit     = 4 << 10
	wsWriteTimeout  = 10 * time.Second
	wsPingInterval  = 20 * time.Second
	wsPingTimeout   = 10 * time.Second
	wsCommandBurst  = 20
	wsCommandWindow = time.Second
)

// WSHandler serves GET /ws (contract §8): cookie auth, Origin check,
// snapshot first, one writer goroutine per connection with a bounded queue.
type WSHandler struct {
	Auth *Authenticator
	Hub  *rooms.Hub
	// AllowedOrigins are exact scheme://host:port values. A browser Origin
	// equal to the request's own scheme://Host is always accepted (same-origin),
	// which covers a page and its API served from the same address on a LAN.
	AllowedOrigins []string
}

func (h *WSHandler) originAllowed(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true // not a browser; the cookie is still required
	}
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	if u.Host == r.Host {
		return true
	}
	for _, allowed := range h.AllowedOrigins {
		if strings.EqualFold(allowed, origin) {
			return true
		}
	}
	return false
}

type wsClient struct {
	conn *websocket.Conn
	out  chan []byte
	once sync.Once
}

func (c *wsClient) Send(msg []byte) bool {
	select {
	case c.out <- msg:
		return true
	default:
		return false
	}
}

// Close never blocks: the close handshake runs in its own goroutine.
func (c *wsClient) Close(code int, reason string) {
	c.once.Do(func() {
		go func() { _ = c.conn.Close(websocket.StatusCode(code), reason) }()
	})
}

func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g, ok := h.Auth.Guest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "сначала войдите: POST /api/guest")
		return
	}
	if !h.originAllowed(r) {
		writeError(w, http.StatusForbidden, "forbidden_origin", "Origin не разрешён")
		return
	}

	// The server-wide Read/WriteTimeout must not outlive the upgrade.
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Time{})
	_ = rc.SetWriteDeadline(time.Time{})

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true}) // Origin checked above
	if err != nil {
		return
	}
	conn.SetReadLimit(wsReadLimit)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := &wsClient{conn: conn, out: make(chan []byte, wsQueueSize)}

	if err := h.Hub.Attach(g.ID, c); err != nil {
		wctx, wcancel := context.WithTimeout(ctx, wsWriteTimeout)
		_ = conn.Write(wctx, websocket.MessageText, []byte(`{"v":1,"type":"error","payload":{"code":"room_expired","message":"вы не в комнате"}}`))
		wcancel()
		_ = conn.Close(rooms.CloseNoRoom, "no room")
		return
	}
	defer func() {
		h.Hub.Detach(g.ID, c)
		_ = conn.CloseNow()
	}()

	go c.writeLoop(ctx, cancel)

	windowStart, count := time.Now(), 0
	for {
		typ, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		if typ != websocket.MessageText {
			continue
		}
		if now := time.Now(); now.Sub(windowStart) > wsCommandWindow {
			windowStart, count = now, 0
		}
		if count++; count > wsCommandBurst {
			c.Send([]byte(`{"v":1,"type":"error","payload":{"code":"rate_limited","message":"слишком много команд"}}`))
			continue
		}
		h.Hub.Command(g.ID, c, data)
	}
}

func (c *wsClient) writeLoop(ctx context.Context, cancel context.CancelFunc) {
	ping := time.NewTicker(wsPingInterval)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-c.out:
			wctx, wcancel := context.WithTimeout(ctx, wsWriteTimeout)
			err := c.conn.Write(wctx, websocket.MessageText, msg)
			wcancel()
			if err != nil {
				cancel()
				return
			}
		case <-ping.C:
			pctx, pcancel := context.WithTimeout(ctx, wsPingTimeout)
			err := c.conn.Ping(pctx)
			pcancel()
			if err != nil {
				cancel()
				return
			}
		}
	}
}
