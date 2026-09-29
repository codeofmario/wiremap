package ws

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/codeofmario/wiremap/internal/wiremap/config"
	"github.com/codeofmario/wiremap/internal/wiremap/dto"
	apperrors "github.com/codeofmario/wiremap/internal/wiremap/errors"
	"github.com/gorilla/websocket"
	"k8s.io/client-go/tools/remotecommand"
)

const testTimeout = 2 * time.Second

// Disconnects and rejected upgrades are logged by design; keep test output readable
func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

// fakePodService lets each test script the pod streams the hub consumes.
type fakePodService struct {
	logs    func(ctx context.Context, cluster, namespace, pod, container string, tail int64) (io.ReadCloser, error)
	metrics func(ctx context.Context, cluster, namespace, pod, container string) (*dto.StatsDto, error)
	exec    func(ctx context.Context, streams remotecommand.StreamOptions) error
}

func (f *fakePodService) Inspect(context.Context, string, string, string) (*dto.KubePodDto, error) {
	return nil, errors.New("not used")
}

func (f *fakePodService) Logs(ctx context.Context, cluster, namespace, pod, container string, tail int64) (io.ReadCloser, error) {
	return f.logs(ctx, cluster, namespace, pod, container, tail)
}

func (f *fakePodService) Metrics(ctx context.Context, cluster, namespace, pod, container string) (*dto.StatsDto, error) {
	return f.metrics(ctx, cluster, namespace, pod, container)
}

func (f *fakePodService) Exec(ctx context.Context, _, _, _, _ string, streams remotecommand.StreamOptions) error {
	return f.exec(ctx, streams)
}

type watchCall struct {
	ctx       context.Context
	cluster   string
	namespace string
	onChange  func()
}

// fakeWatchService records each Watch call; like the real one it returns right away.
type fakeWatchService struct {
	calls chan watchCall
	err   error
}

func newFakeWatchService() *fakeWatchService {
	return &fakeWatchService{calls: make(chan watchCall, 4)}
}

func (f *fakeWatchService) Watch(ctx context.Context, cluster, namespace string, onChange func()) error {
	if f.err != nil {
		return f.err
	}
	f.calls <- watchCall{ctx: ctx, cluster: cluster, namespace: namespace, onChange: onChange}
	return nil
}

type event struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

func newKubeHubServer(t *testing.T, pods *fakePodService, watches *fakeWatchService) *httptest.Server {
	t.Helper()
	hub := NewKubeHub(pods, watches, &config.Settings{})
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/k8s", hub.HandleWS)
	mux.HandleFunc("/ws/k8s/exec", func(w http.ResponseWriter, r *http.Request) {
		hub.HandleExecWS(w, r, "c", "shop", "web-a", "app")
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func dial(t *testing.T, server *httptest.Server, path string, header http.Header) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+path, header)
	if err != nil {
		t.Fatalf("dial %s: %v", path, err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func send(t *testing.T, conn *websocket.Conn, name string, data interface{}) {
	t.Helper()
	if err := conn.WriteJSON(map[string]interface{}{"event": name, "data": data}); err != nil {
		t.Fatalf("send %s: %v", name, err)
	}
}

func receive(t *testing.T, conn *websocket.Conn) event {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(testTimeout))
	var e event
	if err := conn.ReadJSON(&e); err != nil {
		t.Fatalf("receive: %v", err)
	}
	return e
}

func decode[T any](t *testing.T, raw json.RawMessage) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return v
}

func waitDone(t *testing.T, ctx context.Context, what string) {
	t.Helper()
	select {
	case <-ctx.Done():
	case <-time.After(testTimeout):
		t.Fatalf("%s was not cancelled", what)
	}
}

func nextWatch(t *testing.T, watches *fakeWatchService) watchCall {
	t.Helper()
	select {
	case call := <-watches.calls:
		return call
	case <-time.After(testTimeout):
		t.Fatal("Watch was not called")
		return watchCall{}
	}
}

var webPod = podPayload{Cluster: "c", Namespace: "shop", Pod: "web-a", Container: "app"}

// blockingLogs serves lines, then keeps the stream open until the subscription is cancelled.
func blockingLogs(lines string, cancelled chan<- struct{}) func(context.Context, string, string, string, string, int64) (io.ReadCloser, error) {
	return func(ctx context.Context, _, _, _, _ string, _ int64) (io.ReadCloser, error) {
		reader, writer := io.Pipe()
		go func() {
			writer.Write([]byte(lines))
			<-ctx.Done()
			close(cancelled)
			writer.Close()
		}()
		return reader, nil
	}
}

func TestPodLogsStreamLinesUntilUnsubscribed(t *testing.T) {
	cancelled := make(chan struct{})
	var gotTail int64
	pods := &fakePodService{logs: func(ctx context.Context, cluster, namespace, pod, container string, tail int64) (io.ReadCloser, error) {
		if cluster != "c" || namespace != "shop" || pod != "web-a" || container != "app" {
			t.Errorf("logs requested for %s/%s/%s/%s", cluster, namespace, pod, container)
		}
		gotTail = tail
		return blockingLogs("first\nsecond\n", cancelled)(ctx, cluster, namespace, pod, container, tail)
	}}
	conn := dial(t, newKubeHubServer(t, pods, newFakeWatchService()), "/ws/k8s", nil)

	send(t, conn, "pod-logs:subscribe", webPod)
	for _, want := range []string{"first\n", "second\n"} {
		e := receive(t, conn)
		entry := decode[dto.LogEntryDto](t, e.Data)
		if e.Event != "pod-logs:data" || entry.Text != want || entry.ContainerID != "c/shop/web-a/app" || entry.Type != "stdout" {
			t.Errorf("unexpected log event %s %+v", e.Event, entry)
		}
	}
	if gotTail != podLogTailLines {
		t.Errorf("tail = %d, want %d", gotTail, podLogTailLines)
	}

	send(t, conn, "pod-logs:unsubscribe", webPod)
	select {
	case <-cancelled:
	case <-time.After(testTimeout):
		t.Fatal("log stream was not cancelled on unsubscribe")
	}
}

func TestPodLogsReportErrors(t *testing.T) {
	pods := &fakePodService{logs: func(context.Context, string, string, string, string, int64) (io.ReadCloser, error) {
		return nil, apperrors.NotFound("pod web-a not found")
	}}
	conn := dial(t, newKubeHubServer(t, pods, newFakeWatchService()), "/ws/k8s", nil)

	send(t, conn, "pod-logs:subscribe", webPod)
	e := receive(t, conn)
	data := decode[map[string]string](t, e.Data)
	if e.Event != "pod-logs:error" || data["key"] != "c/shop/web-a/app" || data["message"] != "pod web-a not found" {
		t.Errorf("unexpected error event %s %v", e.Event, data)
	}
}

func TestPodStatsSendsMetrics(t *testing.T) {
	pods := &fakePodService{metrics: func(context.Context, string, string, string, string) (*dto.StatsDto, error) {
		return &dto.StatsDto{CPUPercent: 12.5, MemoryUsage: 1024}, nil
	}}
	conn := dial(t, newKubeHubServer(t, pods, newFakeWatchService()), "/ws/k8s", nil)

	send(t, conn, "pod-stats:subscribe", webPod)
	e := receive(t, conn)
	data := decode[struct {
		Key   string       `json:"key"`
		Stats dto.StatsDto `json:"stats"`
	}](t, e.Data)
	if e.Event != "pod-stats:data" || data.Key != "c/shop/web-a/app" || data.Stats.CPUPercent != 12.5 || data.Stats.MemoryUsage != 1024 {
		t.Errorf("unexpected stats event %s %+v", e.Event, data)
	}
	send(t, conn, "pod-stats:unsubscribe", webPod)
}

func TestPodStatsStopOnClientErrorsButKeepPollingOnServerErrors(t *testing.T) {
	for _, tc := range []struct {
		name     string
		err      error
		keepsCtx bool
	}{
		{name: "metrics-server missing", err: apperrors.NotFound("metrics API not available"), keepsCtx: false},
		{name: "transient failure", err: apperrors.Internal("timeout"), keepsCtx: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			polled := make(chan context.Context, 1)
			pods := &fakePodService{metrics: func(ctx context.Context, _, _, _, _ string) (*dto.StatsDto, error) {
				polled <- ctx
				return nil, tc.err
			}}
			conn := dial(t, newKubeHubServer(t, pods, newFakeWatchService()), "/ws/k8s", nil)

			send(t, conn, "pod-stats:subscribe", webPod)
			ctx := <-polled
			e := receive(t, conn)
			data := decode[map[string]string](t, e.Data)
			if e.Event != "pod-stats:error" || data["message"] != tc.err.Error() {
				t.Errorf("unexpected event %s %v", e.Event, data)
			}

			// Unsubscribing always ends the poller; a server error must not have ended it earlier
			if ctx.Err() != nil {
				t.Error("stream context cancelled before unsubscribe")
			}
			send(t, conn, "pod-stats:unsubscribe", webPod)
			waitDone(t, ctx, "stats stream")
		})
	}
}

func TestTopologyWatchForwardsChangesAndStopsOnUnwatch(t *testing.T) {
	watches := newFakeWatchService()
	conn := dial(t, newKubeHubServer(t, &fakePodService{}, watches), "/ws/k8s", nil)

	send(t, conn, "topology:watch", map[string]string{"cluster": "c", "namespace": "shop"})
	call := nextWatch(t, watches)
	if call.cluster != "c" || call.namespace != "shop" {
		t.Errorf("watching %s/%s", call.cluster, call.namespace)
	}

	call.onChange()
	e := receive(t, conn)
	if data := decode[map[string]string](t, e.Data); e.Event != "topology:changed" || data["key"] != "c/shop" {
		t.Errorf("unexpected event %s %v", e.Event, data)
	}

	send(t, conn, "topology:unwatch", map[string]string{"cluster": "c", "namespace": "shop"})
	waitDone(t, call.ctx, "topology watch")
}

func TestTopologyWatchOfClusterLensesUsesEmptyNamespaceKey(t *testing.T) {
	watches := newFakeWatchService()
	conn := dial(t, newKubeHubServer(t, &fakePodService{}, watches), "/ws/k8s", nil)

	send(t, conn, "topology:watch", map[string]string{"cluster": "c"})
	nextWatch(t, watches).onChange()
	if data := decode[map[string]string](t, receive(t, conn).Data); data["key"] != "c/" {
		t.Errorf("cluster lens key = %q", data["key"])
	}
}

func TestTopologyWatchReplacesPreviousWatchOfSameScope(t *testing.T) {
	watches := newFakeWatchService()
	conn := dial(t, newKubeHubServer(t, &fakePodService{}, watches), "/ws/k8s", nil)

	scope := map[string]string{"cluster": "c", "namespace": "shop"}
	send(t, conn, "topology:watch", scope)
	first := nextWatch(t, watches)
	send(t, conn, "topology:watch", scope)
	second := nextWatch(t, watches)

	waitDone(t, first.ctx, "replaced watch")
	if second.ctx.Err() != nil {
		t.Error("the new watch should stay active")
	}
}

func TestTopologyWatchReportsErrors(t *testing.T) {
	watches := newFakeWatchService()
	watches.err = apperrors.NotFound("cluster c is not configured")
	conn := dial(t, newKubeHubServer(t, &fakePodService{}, watches), "/ws/k8s", nil)

	send(t, conn, "topology:watch", map[string]string{"cluster": "c", "namespace": "shop"})
	e := receive(t, conn)
	data := decode[map[string]string](t, e.Data)
	if e.Event != "topology:error" || data["key"] != "c/shop" || data["message"] != "cluster c is not configured" {
		t.Errorf("unexpected event %s %v", e.Event, data)
	}
}

func TestDisconnectCancelsEveryStream(t *testing.T) {
	logsCancelled := make(chan struct{})
	pods := &fakePodService{logs: blockingLogs("", logsCancelled)}
	watches := newFakeWatchService()
	conn := dial(t, newKubeHubServer(t, pods, watches), "/ws/k8s", nil)

	send(t, conn, "pod-logs:subscribe", webPod)
	send(t, conn, "topology:watch", map[string]string{"cluster": "c", "namespace": "shop"})
	watch := nextWatch(t, watches)

	conn.Close()
	waitDone(t, watch.ctx, "topology watch")
	select {
	case <-logsCancelled:
	case <-time.After(testTimeout):
		t.Fatal("log stream was not cancelled on disconnect")
	}
}

func TestMalformedMessagesAreIgnored(t *testing.T) {
	logsCalled := make(chan struct{}, 1)
	pods := &fakePodService{logs: func(context.Context, string, string, string, string, int64) (io.ReadCloser, error) {
		logsCalled <- struct{}{}
		return io.NopCloser(strings.NewReader("")), nil
	}}
	watches := newFakeWatchService()
	conn := dial(t, newKubeHubServer(t, pods, watches), "/ws/k8s", nil)

	conn.WriteMessage(websocket.TextMessage, []byte("not json"))
	conn.WriteMessage(websocket.TextMessage, []byte(`{"event":"pod-logs:subscribe","data":"not an object"}`))
	send(t, conn, "unknown:event", webPod)

	// The connection is still served: a valid message after the bad ones works
	send(t, conn, "topology:watch", map[string]string{"cluster": "c", "namespace": "shop"})
	nextWatch(t, watches)
	select {
	case <-logsCalled:
		t.Error("malformed subscribe should not start a log stream")
	default:
	}
}

func TestUpgradeRejectsForeignOrigins(t *testing.T) {
	idleShell := &fakePodService{exec: func(_ context.Context, streams remotecommand.StreamOptions) error {
		_, err := io.Copy(io.Discard, streams.Stdin)
		return err
	}}
	server := newKubeHubServer(t, idleShell, newFakeWatchService())
	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/k8s"

	_, resp, err := websocket.DefaultDialer.Dial(url, http.Header{"Origin": {"http://evil.example"}})
	if err == nil || resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("foreign origin should get 403, got err=%v resp=%v", err, resp)
	}

	sameOrigin := http.Header{"Origin": {server.URL}}
	dial(t, server, "/ws/k8s", sameOrigin)
	dial(t, server, "/ws/k8s/exec", sameOrigin)
}

func TestExecBridgesStdinStdoutAndResizes(t *testing.T) {
	sizes := make(chan remotecommand.TerminalSize, 1)
	pods := &fakePodService{exec: func(ctx context.Context, streams remotecommand.StreamOptions) error {
		if !streams.Tty {
			t.Error("exec should request a TTY")
		}
		sizes <- *streams.TerminalSizeQueue.Next()
		buf := make([]byte, 64)
		for {
			n, err := streams.Stdin.Read(buf)
			if err != nil {
				return nil
			}
			streams.Stdout.Write(append([]byte("echo:"), buf[:n]...))
		}
	}}
	conn := dial(t, newKubeHubServer(t, pods, newFakeWatchService()), "/ws/k8s/exec", nil)

	conn.WriteMessage(websocket.TextMessage, []byte(`{"event":"resize","cols":120,"rows":40}`))
	select {
	case size := <-sizes:
		if size.Width != 120 || size.Height != 40 {
			t.Errorf("terminal size = %+v", size)
		}
	case <-time.After(testTimeout):
		t.Fatal("resize never reached the exec stream")
	}

	for _, input := range []struct {
		kind int
		text string
	}{
		{websocket.BinaryMessage, "ls\n"},
		// Text frames that aren't resize events are keystrokes too
		{websocket.TextMessage, "pwd\n"},
	} {
		conn.WriteMessage(input.kind, []byte(input.text))
		conn.SetReadDeadline(time.Now().Add(testTimeout))
		kind, out, err := conn.ReadMessage()
		if err != nil || kind != websocket.BinaryMessage || string(out) != "echo:"+input.text {
			t.Errorf("stdout frame = %d %q %v", kind, out, err)
		}
	}
}

func TestExecShowsErrorsAndClosesTheSession(t *testing.T) {
	pods := &fakePodService{exec: func(context.Context, remotecommand.StreamOptions) error {
		return errors.New("container not running")
	}}
	conn := dial(t, newKubeHubServer(t, pods, newFakeWatchService()), "/ws/k8s/exec", nil)

	conn.SetReadDeadline(time.Now().Add(testTimeout))
	kind, out, err := conn.ReadMessage()
	if err != nil || kind != websocket.BinaryMessage || !strings.Contains(string(out), "container not running") {
		t.Fatalf("error frame = %d %q %v", kind, out, err)
	}
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Error("session should close after exec fails")
	}
}

func TestTerminalSizeQueueKeepsOnlyLatestSize(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	queue := &terminalSizeQueue{ctx: ctx, sizes: make(chan remotecommand.TerminalSize, 1)}

	queue.push(remotecommand.TerminalSize{Width: 80, Height: 24})
	queue.push(remotecommand.TerminalSize{Width: 100, Height: 30})
	if size := queue.Next(); size == nil || size.Width != 100 || size.Height != 30 {
		t.Errorf("Next() = %+v, want the latest size", size)
	}

	cancel()
	if size := queue.Next(); size != nil {
		t.Errorf("Next() after cancel = %+v, want nil", size)
	}
}

func TestWsWriterSendsBinaryFramesAndReportsClosedConnections(t *testing.T) {
	serverConn := make(chan *websocket.Conn, 1)
	var once sync.Once
	upgrader := newUpgrader(false)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		once.Do(func() { serverConn <- conn })
	}))
	t.Cleanup(server.Close)
	client := dial(t, server, "/", nil)
	conn := <-serverConn
	writer := &wsWriter{conn: conn}

	if n, err := writer.Write([]byte("hello")); n != 5 || err != nil {
		t.Fatalf("Write = %d, %v", n, err)
	}
	client.SetReadDeadline(time.Now().Add(testTimeout))
	if kind, msg, err := client.ReadMessage(); err != nil || kind != websocket.BinaryMessage || string(msg) != "hello" {
		t.Errorf("frame = %d %q %v", kind, msg, err)
	}

	conn.Close()
	if n, err := writer.Write([]byte("late")); n != 0 || err == nil {
		t.Errorf("Write on closed conn = %d, %v; want an error", n, err)
	}
}
