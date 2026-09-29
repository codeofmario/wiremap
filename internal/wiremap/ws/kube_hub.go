package ws

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/codeofmario/wiremap/internal/wiremap/config"
	"github.com/codeofmario/wiremap/internal/wiremap/dto"
	apperrors "github.com/codeofmario/wiremap/internal/wiremap/errors"
	"github.com/codeofmario/wiremap/internal/wiremap/service"
	"github.com/gorilla/websocket"
	"k8s.io/client-go/tools/remotecommand"
)

const (
	podLogTailLines   = 100
	podMetricsRefresh = 5 * time.Second
)

// KubeHub serves the Kubernetes WebSocket endpoints: pod log/metrics and
// topology-change subscriptions on /ws/k8s, and interactive exec sessions.
type KubeHub struct {
	podService   service.KubePodService
	watchService service.KubeWatchService
	upgrader     websocket.Upgrader
}

func NewKubeHub(podService service.KubePodService, watchService service.KubeWatchService, settings *config.Settings) *KubeHub {
	return &KubeHub{
		podService:   podService,
		watchService: watchService,
		upgrader:     newUpgrader(settings.DevMode),
	}
}

type kubeClient struct {
	hub     *KubeHub
	conn    *websocket.Conn
	cancels map[string]context.CancelFunc
	mu      sync.Mutex
}

type podPayload struct {
	Cluster   string `json:"cluster"`
	Namespace string `json:"namespace"`
	Pod       string `json:"pod"`
	Container string `json:"container"`
}

// key identifies a subscription; the frontend uses the same format to filter events.
func (p podPayload) key() string {
	return fmt.Sprintf("%s/%s/%s/%s", p.Cluster, p.Namespace, p.Pod, p.Container)
}

func (h *KubeHub) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("k8s ws upgrade error: %s", err)
		return
	}

	client := &kubeClient{
		hub:     h,
		conn:    conn,
		cancels: make(map[string]context.CancelFunc),
	}
	go client.readPump()
}

func (c *kubeClient) readPump() {
	defer func() {
		c.cleanup()
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
				log.Printf("k8s ws read error: %s", err)
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

func (c *kubeClient) pingLoop() {
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

func (c *kubeClient) handleMessage(msg wsMessage) {
	var payload podPayload
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		return
	}

	switch msg.Event {
	case "pod-logs:subscribe":
		go c.streamLogs(payload, c.startStream("logs:"+payload.key()))
	case "pod-logs:unsubscribe":
		c.cancelStream("logs:" + payload.key())
	case "pod-stats:subscribe":
		go c.pollMetrics(payload, c.startStream("stats:"+payload.key()))
	case "pod-stats:unsubscribe":
		c.cancelStream("stats:" + payload.key())
	case "topology:watch":
		c.watchTopology(payload, c.startStream("topology:"+payload.topologyKey()))
	case "topology:unwatch":
		c.cancelStream("topology:" + payload.topologyKey())
	}
}

// topologyKey identifies a canvas scope: a namespace, or "" for the cluster lenses.
func (p podPayload) topologyKey() string {
	return p.Cluster + "/" + p.Namespace
}

func (c *kubeClient) watchTopology(payload podPayload, ctx context.Context) {
	key := payload.topologyKey()
	err := c.hub.watchService.Watch(ctx, payload.Cluster, payload.Namespace, func() {
		c.sendJSON("topology:changed", map[string]string{"key": key})
	})
	if err != nil {
		c.sendJSON("topology:error", map[string]string{"key": key, "message": err.Error()})
	}
}

func (c *kubeClient) streamLogs(payload podPayload, ctx context.Context) {
	key := payload.key()
	reader, err := c.hub.podService.Logs(ctx, payload.Cluster, payload.Namespace, payload.Pod, payload.Container, podLogTailLines)
	if err != nil {
		c.sendJSON("pod-logs:error", map[string]string{"key": key, "message": err.Error()})
		return
	}
	defer reader.Close()

	lines := bufio.NewReader(reader)
	for {
		line, err := lines.ReadString('\n')
		if line != "" {
			c.sendJSON("pod-logs:data", dto.LogEntryDto{ContainerID: key, Type: "stdout", Text: line})
		}
		if err != nil {
			return
		}
	}
}

func (c *kubeClient) pollMetrics(payload podPayload, ctx context.Context) {
	key := payload.key()
	ticker := time.NewTicker(podMetricsRefresh)
	defer ticker.Stop()

	for {
		stats, err := c.hub.podService.Metrics(ctx, payload.Cluster, payload.Namespace, payload.Pod, payload.Container)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			c.sendJSON("pod-stats:error", map[string]string{"key": key, "message": err.Error()})
			// Missing metrics-server or RBAC won't fix themselves; stop polling
			var appErr *apperrors.AppError
			if errors.As(err, &appErr) && appErr.StatusCode < http.StatusInternalServerError {
				return
			}
		} else {
			c.sendJSON("pod-stats:data", map[string]interface{}{"key": key, "stats": stats})
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (c *kubeClient) sendJSON(event string, data interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.conn.WriteJSON(map[string]interface{}{"event": event, "data": data})
}

// startStream replaces any running stream under key and returns the new stream's context.
func (c *kubeClient) startStream(key string) context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	c.mu.Lock()
	defer c.mu.Unlock()
	if previous, ok := c.cancels[key]; ok {
		previous()
	}
	c.cancels[key] = cancel
	return ctx
}

func (c *kubeClient) cancelStream(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if cancel, ok := c.cancels[key]; ok {
		cancel()
		delete(c.cancels, key)
	}
}

func (c *kubeClient) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, cancel := range c.cancels {
		cancel()
	}
	c.cancels = make(map[string]context.CancelFunc)
}

// HandleExecWS bridges an xterm session in the browser to a TTY shell in the pod.
// Binary frames carry stdin/stdout; text frames carry resize events.
func (h *KubeHub) HandleExecWS(w http.ResponseWriter, r *http.Request, cluster, namespace, pod, container string) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("k8s exec ws upgrade error: %s", err)
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stdinReader, stdinWriter := io.Pipe()
	stdout := &wsWriter{conn: conn}
	sizes := &terminalSizeQueue{ctx: ctx, sizes: make(chan remotecommand.TerminalSize, 1)}

	go func() {
		err := h.podService.Exec(ctx, cluster, namespace, pod, container, remotecommand.StreamOptions{
			Stdin:             stdinReader,
			Stdout:            stdout,
			Tty:               true,
			TerminalSizeQueue: sizes,
		})
		if err != nil && ctx.Err() == nil {
			stdout.Write([]byte(fmt.Sprintf("\r\n\x1b[31m%s\x1b[0m\r\n", err)))
		}
		conn.Close()
	}()

	for {
		msgType, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}

		if msgType == websocket.TextMessage {
			var cmd struct {
				Event string `json:"event"`
				Cols  uint16 `json:"cols"`
				Rows  uint16 `json:"rows"`
			}
			if json.Unmarshal(msg, &cmd) == nil && cmd.Event == "resize" {
				sizes.push(remotecommand.TerminalSize{Width: cmd.Cols, Height: cmd.Rows})
				continue
			}
		}

		if _, err := stdinWriter.Write(msg); err != nil {
			break
		}
	}

	stdinWriter.Close()
}

// wsWriter forwards exec output to the browser as binary frames.
type wsWriter struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (w *wsWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.conn.WriteMessage(websocket.BinaryMessage, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

// terminalSizeQueue keeps only the latest resize so a slow exec stream never blocks the reader.
type terminalSizeQueue struct {
	ctx   context.Context
	sizes chan remotecommand.TerminalSize
}

func (q *terminalSizeQueue) push(size remotecommand.TerminalSize) {
	select {
	case <-q.sizes:
	default:
	}
	q.sizes <- size
}

func (q *terminalSizeQueue) Next() *remotecommand.TerminalSize {
	select {
	case size := <-q.sizes:
		return &size
	case <-q.ctx.Done():
		return nil
	}
}
