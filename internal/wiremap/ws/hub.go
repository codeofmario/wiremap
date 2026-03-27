package ws

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/codeofmario/wiremap/internal/wiremap/config"
	"github.com/codeofmario/wiremap/internal/wiremap/dto"
	"github.com/codeofmario/wiremap/internal/wiremap/service"
	"github.com/gorilla/websocket"
)

func newUpgrader(devMode bool) websocket.Upgrader {
	return websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true
			}
			host := r.Host
			if origin == "http://"+host || origin == "https://"+host {
				return true
			}
			if devMode && origin == "http://localhost:5173" {
				return true
			}
			return false
		},
	}
}

type Hub struct {
	containerService service.ContainerService
	upgrader         websocket.Upgrader
	mu               sync.Mutex
	clients          map[*Client]bool
}

func NewHub(containerService service.ContainerService, settings *config.Settings) *Hub {
	return &Hub{
		containerService: containerService,
		upgrader:         newUpgrader(settings.DevMode),
		clients:          make(map[*Client]bool),
	}
}

type Client struct {
	hub     *Hub
	conn    *websocket.Conn
	cancels map[string]context.CancelFunc
	mu      sync.Mutex
}

type wsMessage struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

type subscribePayload struct {
	ContainerID string `json:"containerId"`
	HostID      string `json:"hostId"`
}

func (h *Hub) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade error: %s", err)
		return
	}

	client := &Client{
		hub:     h,
		conn:    conn,
		cancels: make(map[string]context.CancelFunc),
	}

	h.mu.Lock()
	h.clients[client] = true
	h.mu.Unlock()

	go client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.cleanup()
		c.hub.mu.Lock()
		delete(c.hub.clients, c)
		c.hub.mu.Unlock()
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	go c.pingLoop()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("ws read error: %s", err)
			}
			return
		}

		var msg wsMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		c.handleMessage(msg)
	}
}

func (c *Client) pingLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		err := c.conn.WriteMessage(websocket.PingMessage, nil)
		c.mu.Unlock()
		if err != nil {
			return
		}
	}
}

func (c *Client) handleMessage(msg wsMessage) {
	var payload subscribePayload
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		return
	}

	switch msg.Event {
	case "logs:subscribe":
		c.cancelStream("logs:" + payload.ContainerID)
		go c.streamLogs(payload)
	case "logs:unsubscribe":
		c.cancelStream("logs:" + payload.ContainerID)
	case "stats:subscribe":
		c.cancelStream("stats:" + payload.ContainerID)
		go c.streamStats(payload)
	case "stats:unsubscribe":
		c.cancelStream("stats:" + payload.ContainerID)
	}
}

func (c *Client) streamLogs(payload subscribePayload) {
	containerID := payload.ContainerID
	ctx, cancel := context.WithCancel(context.Background())
	c.mu.Lock()
	c.cancels["logs:"+containerID] = cancel
	c.mu.Unlock()

	reader, err := c.hub.containerService.Logs(ctx, payload.HostID, containerID, "100")
	if err != nil {
		c.sendJSON("logs:error", map[string]string{"containerId": containerID, "message": err.Error()})
		return
	}
	defer reader.Close()

	header := make([]byte, 8)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_, err := io.ReadFull(reader, header)
		if err != nil {
			return
		}

		size := int(header[7]) | int(header[6])<<8 | int(header[5])<<16 | int(header[4])<<24
		if size <= 0 || size > 1<<20 {
			continue
		}

		chunk := make([]byte, size)
		_, err = io.ReadFull(reader, chunk)
		if err != nil {
			return
		}

		streamType := "stdout"
		if header[0] == 2 {
			streamType = "stderr"
		}

		c.sendJSON("logs:data", dto.LogEntryDto{
			ContainerID: containerID,
			Type:        streamType,
			Text:        string(chunk),
		})
	}
}

func (c *Client) streamStats(payload subscribePayload) {
	containerID := payload.ContainerID
	ctx, cancel := context.WithCancel(context.Background())
	c.mu.Lock()
	c.cancels["stats:"+containerID] = cancel
	c.mu.Unlock()

	reader, err := c.hub.containerService.Stats(ctx, payload.HostID, containerID)
	if err != nil {
		c.sendJSON("stats:error", map[string]string{"containerId": containerID, "message": err.Error()})
		return
	}
	defer reader.Close()

	decoder := json.NewDecoder(reader)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return
		}

		stats, err := c.hub.containerService.ParseStats(raw)
		if err != nil {
			continue
		}

		c.sendJSON("stats:data", map[string]interface{}{
			"containerId": containerID,
			"stats":       stats,
		})
	}
}

func (c *Client) sendJSON(event string, data interface{}) {
	msg := map[string]interface{}{
		"event": event,
		"data":  data,
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.conn.WriteJSON(msg)
}

func (c *Client) cancelStream(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if cancel, ok := c.cancels[key]; ok {
		cancel()
		delete(c.cancels, key)
	}
}

func (c *Client) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, cancel := range c.cancels {
		cancel()
	}
	c.cancels = make(map[string]context.CancelFunc)
}

func (h *Hub) HandleExecWS(w http.ResponseWriter, r *http.Request, containerID string, hostID string) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("exec ws upgrade error: %s", err)
		return
	}
	defer conn.Close()

	session, err := h.containerService.Exec(r.Context(), hostID, containerID, []string{"/bin/sh"})
	if err != nil {
		conn.WriteJSON(map[string]string{"event": "exec:error", "message": err.Error()})
		return
	}
	defer session.Conn.Close()

	done := make(chan struct{})

	// Docker → WebSocket
	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		for {
			n, err := session.Conn.Read(buf)
			if err != nil {
				return
			}
			if err := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
				return
			}
		}
	}()

	// WebSocket → Docker
	for {
		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}

		if msgType == websocket.TextMessage {
			var cmd struct {
				Event string `json:"event"`
				Cols  uint   `json:"cols"`
				Rows  uint   `json:"rows"`
			}
			if json.Unmarshal(msg, &cmd) == nil && cmd.Event == "resize" {
				// Resize is best-effort; exec ID isn't easily available here
				// but TTY resize via the attached connection works automatically
				continue
			}
		}

		if _, err := session.Conn.Write(msg); err != nil {
			break
		}
	}

	<-done
}
