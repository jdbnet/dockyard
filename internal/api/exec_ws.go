package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

type wsResizeMsg struct {
	Type string `json:"type"`
	Cols uint   `json:"cols"`
	Rows uint   `json:"rows"`
}

type websocketTerminalSession struct {
	conn     *websocket.Conn
	writeMu  sync.Mutex
	pending  []byte
	resizeFn func(cols, rows uint)
}

func newWebsocketTerminalSession(conn *websocket.Conn) *websocketTerminalSession {
	return &websocketTerminalSession{conn: conn}
}

func (s *websocketTerminalSession) Read(p []byte) (int, error) {
	if len(s.pending) > 0 {
		n := copy(p, s.pending)
		s.pending = s.pending[n:]
		return n, nil
	}
	for {
		msgType, data, err := s.conn.ReadMessage()
		if err != nil {
			return 0, err
		}
		switch msgType {
		case websocket.BinaryMessage:
			if len(data) == 0 {
				continue
			}
			n := copy(p, data)
			if n < len(data) {
				s.pending = data[n:]
			}
			return n, nil
		case websocket.TextMessage:
			var msg wsResizeMsg
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}
			if msg.Type == "resize" && s.resizeFn != nil && msg.Cols > 0 && msg.Rows > 0 {
				s.resizeFn(msg.Cols, msg.Rows)
			}
		default:
			continue
		}
	}
}

func (s *websocketTerminalSession) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	s.writeMu.Lock()
	err := s.conn.WriteMessage(websocket.BinaryMessage, p)
	s.writeMu.Unlock()
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (s *websocketTerminalSession) Size() (cols, rows uint) {
	return 80, 24
}

func (s *websocketTerminalSession) OnResize(fn func(cols, rows uint)) {
	s.resizeFn = fn
}

func (s *Server) handleWSExec(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ctx := r.Context()
	list, _ := s.eng.Containers(ctx)
	id = matchContainerID(list, id)

	session := newWebsocketTerminalSession(conn)
	err = s.eng.Shell(ctx, id, session)
	if err != nil && !isClosedConn(err) {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("error: "+err.Error()))
	}
}

func isClosedConn(err error) bool {
	if err == nil {
		return false
	}
	if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
		return true
	}
	if err == io.EOF {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "use of closed network connection") ||
		strings.Contains(msg, "connection reset by peer")
}
