package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/codeofmario/wiremap/internal/wiremap/dto"
	apperrors "github.com/codeofmario/wiremap/internal/wiremap/errors"
	"github.com/codeofmario/wiremap/internal/wiremap/service"
	"github.com/gin-gonic/gin"
	"k8s.io/client-go/tools/remotecommand"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// serve registers one handler on a fresh router and records the response to a request.
func serve(method, route, target, body string, handle gin.HandlerFunc) *httptest.ResponseRecorder {
	r := gin.New()
	r.Handle(method, route, handle)
	w := httptest.NewRecorder()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func decode[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatalf("invalid JSON %q: %v", w.Body.String(), err)
	}
	return v
}

func assertStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("status = %d, want %d (body %s)", w.Code, want, w.Body.String())
	}
}

// Service fakes record their arguments and return the configured result.

type fakeClusterService struct{ clusters []dto.KubeClusterDto }

func (f *fakeClusterService) List() []dto.KubeClusterDto { return f.clusters }

type topologyCall struct {
	method                                      string
	cluster, namespace, group, kind, name, lens string
}

type fakeTopologyService struct {
	call  topologyCall
	graph *dto.KubeGraphDto
	err   error
}

func (f *fakeTopologyService) Cluster(_ context.Context, cluster, lens string) (*dto.KubeGraphDto, error) {
	f.call = topologyCall{method: "Cluster", cluster: cluster, lens: lens}
	return f.graph, f.err
}

func (f *fakeTopologyService) Namespace(_ context.Context, cluster, namespace string) (*dto.KubeGraphDto, error) {
	f.call = topologyCall{method: "Namespace", cluster: cluster, namespace: namespace}
	return f.graph, f.err
}

func (f *fakeTopologyService) Object(_ context.Context, cluster, namespace, group, kind, name string) (*dto.KubeGraphDto, error) {
	f.call = topologyCall{method: "Object", cluster: cluster, namespace: namespace, group: group, kind: kind, name: name}
	return f.graph, f.err
}

func (f *fakeTopologyService) Node(_ context.Context, cluster, node string) (*dto.KubeGraphDto, error) {
	f.call = topologyCall{method: "Node", cluster: cluster, name: node}
	return f.graph, f.err
}

type fakeBrowserService struct {
	kinds     []dto.KubeKindDto
	list      *dto.KubeObjectListDto
	counts    map[string]int
	err       error
	listArgs  []string
	namespace string
	kindsArg  []string
}

func (f *fakeBrowserService) Kinds(_ context.Context, cluster string) ([]dto.KubeKindDto, error) {
	return f.kinds, f.err
}

func (f *fakeBrowserService) List(_ context.Context, cluster, group, version, resource, namespace, continueToken string) (*dto.KubeObjectListDto, error) {
	f.listArgs = []string{cluster, group, version, resource, namespace, continueToken}
	return f.list, f.err
}

func (f *fakeBrowserService) Counts(_ context.Context, cluster, namespace string, kinds []string) (map[string]int, error) {
	f.namespace, f.kindsArg = namespace, kinds
	return f.counts, f.err
}

type fakePodService struct {
	args []string
	pod  *dto.KubePodDto
	err  error
}

func (f *fakePodService) Inspect(_ context.Context, cluster, namespace, name string) (*dto.KubePodDto, error) {
	f.args = []string{cluster, namespace, name}
	return f.pod, f.err
}

func (f *fakePodService) Logs(context.Context, string, string, string, string, int64) (io.ReadCloser, error) {
	return nil, nil
}

func (f *fakePodService) Metrics(context.Context, string, string, string, string) (*dto.StatsDto, error) {
	return nil, nil
}

func (f *fakePodService) Exec(context.Context, string, string, string, string, remotecommand.StreamOptions) error {
	return nil
}

type fakeResourceService struct {
	args     []string
	resource *dto.KubeResourceDto
	err      error
}

func (f *fakeResourceService) Get(_ context.Context, cluster, group, kind, namespace, name string) (*dto.KubeResourceDto, error) {
	f.args = []string{cluster, group, kind, namespace, name}
	return f.resource, f.err
}

type fakeActionService struct {
	method    string
	ref       service.ObjectRef
	manifest  string
	replicas  int32
	suspended bool
	job       string
	err       error
}

func (f *fakeActionService) Update(_ context.Context, ref service.ObjectRef, manifest string) error {
	f.method, f.ref, f.manifest = "Update", ref, manifest
	return f.err
}

func (f *fakeActionService) Delete(_ context.Context, ref service.ObjectRef) error {
	f.method, f.ref = "Delete", ref
	return f.err
}

func (f *fakeActionService) Scale(_ context.Context, ref service.ObjectRef, replicas int32) error {
	f.method, f.ref, f.replicas = "Scale", ref, replicas
	return f.err
}

func (f *fakeActionService) Restart(_ context.Context, ref service.ObjectRef) error {
	f.method, f.ref = "Restart", ref
	return f.err
}

func (f *fakeActionService) TriggerCronJob(_ context.Context, ref service.ObjectRef) (string, error) {
	f.method, f.ref = "TriggerCronJob", ref
	return f.job, f.err
}

func (f *fakeActionService) SetSuspended(_ context.Context, ref service.ObjectRef, suspended bool) error {
	f.method, f.ref, f.suspended = "SetSuspended", ref, suspended
	return f.err
}

type fakeNodeActionService struct {
	method        string
	cluster, node string
	unschedulable bool
	refused       []string
	err           error
}

func (f *fakeNodeActionService) SetUnschedulable(_ context.Context, cluster, node string, unschedulable bool) error {
	f.method, f.cluster, f.node, f.unschedulable = "SetUnschedulable", cluster, node, unschedulable
	return f.err
}

func (f *fakeNodeActionService) Drain(_ context.Context, cluster, node string) ([]string, error) {
	f.method, f.cluster, f.node = "Drain", cluster, node
	return f.refused, f.err
}

func TestClusterListReturnsClusters(t *testing.T) {
	clusters := []dto.KubeClusterDto{{Name: "prod", Connected: true}, {Name: "dev"}}
	h := NewKubeClusterHandler(&fakeClusterService{clusters: clusters}, &fakeTopologyService{})

	w := serve(http.MethodGet, "/clusters", "/clusters", "", h.List)

	assertStatus(t, w, http.StatusOK)
	if got := decode[[]dto.KubeClusterDto](t, w); !reflect.DeepEqual(got, clusters) {
		t.Errorf("clusters = %+v", got)
	}
}

func TestTopologyDispatchesOnLevel(t *testing.T) {
	const route = "/clusters/:cluster/topology"
	tests := []struct {
		name  string
		query string
		want  topologyCall
	}{
		{"defaults to cluster namespaces lens", "", topologyCall{method: "Cluster", cluster: "prod", lens: service.LensNamespaces}},
		{"cluster with lens", "?level=cluster&lens=nodes", topologyCall{method: "Cluster", cluster: "prod", lens: "nodes"}},
		{"namespace", "?level=namespace&namespace=shop", topologyCall{method: "Namespace", cluster: "prod", namespace: "shop"}},
		{
			"object", "?level=object&namespace=shop&group=apps&kind=Deployment&name=api",
			topologyCall{method: "Object", cluster: "prod", namespace: "shop", group: "apps", kind: "Deployment", name: "api"},
		},
		{"node", "?level=node&name=worker-1", topologyCall{method: "Node", cluster: "prod", name: "worker-1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topology := &fakeTopologyService{graph: &dto.KubeGraphDto{Level: "x", Nodes: []dto.KubeGraphNodeDto{{ID: "Pod/shop/a"}}}}
			h := NewKubeClusterHandler(&fakeClusterService{}, topology)

			w := serve(http.MethodGet, route, "/clusters/prod/topology"+tt.query, "", h.Topology)

			assertStatus(t, w, http.StatusOK)
			if topology.call != tt.want {
				t.Errorf("call = %+v, want %+v", topology.call, tt.want)
			}
			if got := decode[dto.KubeGraphDto](t, w); len(got.Nodes) != 1 || got.Nodes[0].ID != "Pod/shop/a" {
				t.Errorf("graph = %+v", got)
			}
		})
	}
}

func TestTopologyRejectsUnknownLevel(t *testing.T) {
	topology := &fakeTopologyService{}
	h := NewKubeClusterHandler(&fakeClusterService{}, topology)

	w := serve(http.MethodGet, "/clusters/:cluster/topology", "/clusters/prod/topology?level=galaxy", "", h.Topology)

	assertStatus(t, w, http.StatusBadRequest)
	if topology.call.method != "" {
		t.Errorf("service should not be called, got %+v", topology.call)
	}
	if msg := decode[apperrors.AppError](t, w).Message; !strings.Contains(msg, "galaxy") {
		t.Errorf("message = %q", msg)
	}
}

func TestTopologyMapsServiceErrors(t *testing.T) {
	tests := []struct {
		err  error
		want int
	}{
		{apperrors.NotFound("cluster missing"), http.StatusNotFound},
		{apperrors.Forbidden("rbac"), http.StatusForbidden},
		{apperrors.BadRequest("bad"), http.StatusBadRequest},
		{errors.New("boom"), http.StatusInternalServerError},
	}
	for _, tt := range tests {
		h := NewKubeClusterHandler(&fakeClusterService{}, &fakeTopologyService{err: tt.err})
		w := serve(http.MethodGet, "/clusters/:cluster/topology", "/clusters/prod/topology?level=namespace&namespace=x", "", h.Topology)
		assertStatus(t, w, tt.want)
	}
}

func TestBrowserKinds(t *testing.T) {
	kinds := []dto.KubeKindDto{{Group: "apps", Version: "v1", Resource: "deployments", Kind: "Deployment", Namespaced: true, ShortNames: []string{"deploy"}}}
	h := NewKubeBrowserHandler(&fakeBrowserService{kinds: kinds})

	w := serve(http.MethodGet, "/clusters/:cluster/kinds", "/clusters/prod/kinds", "", h.Kinds)

	assertStatus(t, w, http.StatusOK)
	if got := decode[[]dto.KubeKindDto](t, w); !reflect.DeepEqual(got, kinds) {
		t.Errorf("kinds = %+v", got)
	}

	w = serve(http.MethodGet, "/clusters/:cluster/kinds", "/clusters/prod/kinds", "", NewKubeBrowserHandler(&fakeBrowserService{err: apperrors.NotFound("x")}).Kinds)
	assertStatus(t, w, http.StatusNotFound)
}

func TestBrowserListPassesQuery(t *testing.T) {
	browser := &fakeBrowserService{list: &dto.KubeObjectListDto{Items: []dto.KubeObjectDto{{Name: "api", Drillable: true}}, Continue: "next"}}
	h := NewKubeBrowserHandler(browser)

	w := serve(http.MethodGet, "/clusters/:cluster/objects",
		"/clusters/prod/objects?group=apps&version=v1&resource=deployments&namespace=shop&continue=tok", "", h.List)

	assertStatus(t, w, http.StatusOK)
	if want := []string{"prod", "apps", "v1", "deployments", "shop", "tok"}; !reflect.DeepEqual(browser.listArgs, want) {
		t.Errorf("args = %v, want %v", browser.listArgs, want)
	}
	if got := decode[dto.KubeObjectListDto](t, w); got.Continue != "next" || len(got.Items) != 1 || !got.Items[0].Drillable {
		t.Errorf("list = %+v", got)
	}

	w = serve(http.MethodGet, "/clusters/:cluster/objects", "/clusters/prod/objects", "", NewKubeBrowserHandler(&fakeBrowserService{err: apperrors.BadRequest("version and resource are required")}).List)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestBrowserCountsSplitsKinds(t *testing.T) {
	tests := []struct {
		name  string
		query string
		kinds []string
	}{
		{"comma separated", "?namespace=shop&kinds=/Pod,apps/Deployment", []string{"/Pod", "apps/Deployment"}},
		{"single", "?namespace=shop&kinds=/Pod", []string{"/Pod"}},
		{"empty", "?namespace=shop", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			browser := &fakeBrowserService{counts: map[string]int{"/Pod": 3}}
			h := NewKubeBrowserHandler(browser)

			w := serve(http.MethodGet, "/clusters/:cluster/counts", "/clusters/prod/counts"+tt.query, "", h.Counts)

			assertStatus(t, w, http.StatusOK)
			if browser.namespace != "shop" || !reflect.DeepEqual(browser.kindsArg, tt.kinds) {
				t.Errorf("namespace %q kinds %v, want %v", browser.namespace, browser.kindsArg, tt.kinds)
			}
			if got := decode[map[string]int](t, w); got["/Pod"] != 3 {
				t.Errorf("counts = %+v", got)
			}
		})
	}
}

func TestBrowserCountsMapsErrors(t *testing.T) {
	h := NewKubeBrowserHandler(&fakeBrowserService{err: apperrors.BadRequest(`kind "Pod" must be group/Kind`)})
	w := serve(http.MethodGet, "/clusters/:cluster/counts", "/clusters/prod/counts?kinds=Pod", "", h.Counts)
	assertStatus(t, w, http.StatusBadRequest)
}

func TestPodInspect(t *testing.T) {
	pods := &fakePodService{pod: &dto.KubePodDto{Name: "api-1", Namespace: "shop", Phase: "Running"}}
	h := NewKubePodHandler(pods)
	route := "/clusters/:cluster/namespaces/:namespace/pods/:name"

	w := serve(http.MethodGet, route, "/clusters/prod/namespaces/shop/pods/api-1", "", h.Inspect)

	assertStatus(t, w, http.StatusOK)
	if !reflect.DeepEqual(pods.args, []string{"prod", "shop", "api-1"}) {
		t.Errorf("args = %v", pods.args)
	}
	if got := decode[dto.KubePodDto](t, w); got.Name != "api-1" || got.Phase != "Running" {
		t.Errorf("pod = %+v", got)
	}

	w = serve(http.MethodGet, route, "/clusters/prod/namespaces/shop/pods/gone", "", NewKubePodHandler(&fakePodService{err: apperrors.NotFound("gone")}).Inspect)
	assertStatus(t, w, http.StatusNotFound)
}

func TestResourceGet(t *testing.T) {
	resources := &fakeResourceService{resource: &dto.KubeResourceDto{Kind: "Deployment", Group: "apps", Name: "api", YAML: "kind: Deployment"}}
	h := NewKubeResourceHandler(resources)
	route := "/clusters/:cluster/resources/:kind/:name"

	w := serve(http.MethodGet, route, "/clusters/prod/resources/Deployment/api?group=apps&namespace=shop", "", h.Get)

	assertStatus(t, w, http.StatusOK)
	if want := []string{"prod", "apps", "Deployment", "shop", "api"}; !reflect.DeepEqual(resources.args, want) {
		t.Errorf("args = %v, want %v", resources.args, want)
	}
	if got := decode[dto.KubeResourceDto](t, w); got.YAML != "kind: Deployment" {
		t.Errorf("resource = %+v", got)
	}

	w = serve(http.MethodGet, route, "/clusters/prod/resources/Secret/x", "", NewKubeResourceHandler(&fakeResourceService{err: apperrors.Forbidden("rbac")}).Get)
	assertStatus(t, w, http.StatusForbidden)
}

func TestActionsPassObjectRefAndBody(t *testing.T) {
	const base = "/clusters/:cluster/resources/:kind/:name"
	const target = "/clusters/prod/resources/Deployment/api?group=apps&namespace=shop"
	wantRef := service.ObjectRef{Cluster: "prod", Group: "apps", Kind: "Deployment", Namespace: "shop", Name: "api"}

	tests := []struct {
		name       string
		method     string
		route      string
		path       string
		body       string
		handle     func(h *KubeActionHandler) gin.HandlerFunc
		wantStatus int
		check      func(t *testing.T, f *fakeActionService, w *httptest.ResponseRecorder)
	}{
		{
			"update", http.MethodPut, base, target, `{"yaml":"kind: Deployment"}`,
			func(h *KubeActionHandler) gin.HandlerFunc { return h.Update }, http.StatusNoContent,
			func(t *testing.T, f *fakeActionService, _ *httptest.ResponseRecorder) {
				if f.method != "Update" || f.manifest != "kind: Deployment" {
					t.Errorf("got %s %q", f.method, f.manifest)
				}
			},
		},
		{
			"delete", http.MethodDelete, base, target, "",
			func(h *KubeActionHandler) gin.HandlerFunc { return h.Delete }, http.StatusNoContent,
			func(t *testing.T, f *fakeActionService, _ *httptest.ResponseRecorder) {
				if f.method != "Delete" {
					t.Errorf("got %s", f.method)
				}
			},
		},
		{
			"scale to zero", http.MethodPost, base + "/scale", strings.Replace(target, "api?", "api/scale?", 1), `{"replicas":0}`,
			func(h *KubeActionHandler) gin.HandlerFunc { return h.Scale }, http.StatusNoContent,
			func(t *testing.T, f *fakeActionService, _ *httptest.ResponseRecorder) {
				if f.method != "Scale" || f.replicas != 0 {
					t.Errorf("got %s %d", f.method, f.replicas)
				}
			},
		},
		{
			"restart", http.MethodPost, base + "/restart", strings.Replace(target, "api?", "api/restart?", 1), "",
			func(h *KubeActionHandler) gin.HandlerFunc { return h.Restart }, http.StatusNoContent,
			func(t *testing.T, f *fakeActionService, _ *httptest.ResponseRecorder) {
				if f.method != "Restart" {
					t.Errorf("got %s", f.method)
				}
			},
		},
		{
			"trigger", http.MethodPost, base + "/trigger", strings.Replace(target, "api?", "api/trigger?", 1), "",
			func(h *KubeActionHandler) gin.HandlerFunc { return h.Trigger }, http.StatusCreated,
			func(t *testing.T, f *fakeActionService, w *httptest.ResponseRecorder) {
				if got := decode[map[string]string](t, w)["job"]; f.method != "TriggerCronJob" || got != "api-manual-1" {
					t.Errorf("got %s job %q", f.method, got)
				}
			},
		},
		{
			"suspend", http.MethodPost, base + "/suspend", strings.Replace(target, "api?", "api/suspend?", 1), `{"value":true}`,
			func(h *KubeActionHandler) gin.HandlerFunc { return h.Suspend }, http.StatusNoContent,
			func(t *testing.T, f *fakeActionService, _ *httptest.ResponseRecorder) {
				if f.method != "SetSuspended" || !f.suspended {
					t.Errorf("got %s %v", f.method, f.suspended)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actions := &fakeActionService{job: "api-manual-1"}
			h := NewKubeActionHandler(actions, &fakeNodeActionService{})

			w := serve(tt.method, tt.route, tt.path, tt.body, tt.handle(h))

			assertStatus(t, w, tt.wantStatus)
			if actions.ref != wantRef {
				t.Errorf("ref = %+v, want %+v", actions.ref, wantRef)
			}
			tt.check(t, actions, w)
		})
	}
}

func TestActionsRejectMissingBodyFields(t *testing.T) {
	const base = "/clusters/:cluster/resources/:kind/:name"
	tests := []struct {
		name   string
		route  string
		path   string
		body   string
		handle func(h *KubeActionHandler) gin.HandlerFunc
	}{
		{"update without yaml", base, "/clusters/prod/resources/Deployment/api", `{}`, func(h *KubeActionHandler) gin.HandlerFunc { return h.Update }},
		{"scale without replicas", base + "/scale", "/clusters/prod/resources/Deployment/api/scale", `{}`, func(h *KubeActionHandler) gin.HandlerFunc { return h.Scale }},
		{"suspend without value", base + "/suspend", "/clusters/prod/resources/CronJob/job/suspend", `{}`, func(h *KubeActionHandler) gin.HandlerFunc { return h.Suspend }},
		{"invalid json", base + "/scale", "/clusters/prod/resources/Deployment/api/scale", `{`, func(h *KubeActionHandler) gin.HandlerFunc { return h.Scale }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actions := &fakeActionService{}
			h := NewKubeActionHandler(actions, &fakeNodeActionService{})

			w := serve(http.MethodPost, tt.route, tt.path, tt.body, tt.handle(h))

			assertStatus(t, w, http.StatusBadRequest)
			if actions.method != "" {
				t.Errorf("service should not be called, got %s", actions.method)
			}
		})
	}
}

func TestActionsMapServiceErrors(t *testing.T) {
	const route = "/clusters/:cluster/resources/:kind/:name"
	handlers := map[string]func(h *KubeActionHandler) gin.HandlerFunc{
		"update":  func(h *KubeActionHandler) gin.HandlerFunc { return h.Update },
		"delete":  func(h *KubeActionHandler) gin.HandlerFunc { return h.Delete },
		"scale":   func(h *KubeActionHandler) gin.HandlerFunc { return h.Scale },
		"restart": func(h *KubeActionHandler) gin.HandlerFunc { return h.Restart },
		"trigger": func(h *KubeActionHandler) gin.HandlerFunc { return h.Trigger },
		"suspend": func(h *KubeActionHandler) gin.HandlerFunc { return h.Suspend },
	}
	body := `{"yaml":"x","replicas":1,"value":false}`
	for name, handle := range handlers {
		t.Run(name, func(t *testing.T) {
			h := NewKubeActionHandler(&fakeActionService{err: apperrors.Forbidden("rbac denied")}, &fakeNodeActionService{})
			w := serve(http.MethodPost, route, "/clusters/prod/resources/Deployment/api", body, handle(h))
			assertStatus(t, w, http.StatusForbidden)
			if msg := decode[apperrors.AppError](t, w).Message; msg != "rbac denied" {
				t.Errorf("message = %q", msg)
			}
		})
	}
}

func TestCordonAndDrainNodes(t *testing.T) {
	const cordonRoute = "/clusters/:cluster/nodes/:name/cordon"
	const drainRoute = "/clusters/:cluster/nodes/:name/drain"

	nodes := &fakeNodeActionService{}
	h := NewKubeActionHandler(&fakeActionService{}, nodes)
	w := serve(http.MethodPost, cordonRoute, "/clusters/prod/nodes/worker-1/cordon", `{"value":true}`, h.Cordon)
	assertStatus(t, w, http.StatusNoContent)
	if nodes.method != "SetUnschedulable" || nodes.cluster != "prod" || nodes.node != "worker-1" || !nodes.unschedulable {
		t.Errorf("cordon call = %+v", nodes)
	}

	w = serve(http.MethodPost, cordonRoute, "/clusters/prod/nodes/worker-1/cordon", `{}`, h.Cordon)
	assertStatus(t, w, http.StatusBadRequest)

	nodes = &fakeNodeActionService{refused: []string{"shop/api-1"}}
	h = NewKubeActionHandler(&fakeActionService{}, nodes)
	w = serve(http.MethodPost, drainRoute, "/clusters/prod/nodes/worker-1/drain", "", h.Drain)
	assertStatus(t, w, http.StatusOK)
	if got := decode[map[string][]string](t, w)["refused"]; nodes.method != "Drain" || !reflect.DeepEqual(got, []string{"shop/api-1"}) {
		t.Errorf("drain %s refused %v", nodes.method, got)
	}

	failing := NewKubeActionHandler(&fakeActionService{}, &fakeNodeActionService{err: apperrors.NotFound("no node")})
	assertStatus(t, serve(http.MethodPost, cordonRoute, "/clusters/prod/nodes/x/cordon", `{"value":false}`, failing.Cordon), http.StatusNotFound)
	assertStatus(t, serve(http.MethodPost, drainRoute, "/clusters/prod/nodes/x/drain", "", failing.Drain), http.StatusNotFound)
}
