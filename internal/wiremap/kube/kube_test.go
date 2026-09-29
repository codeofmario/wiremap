package kube

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/codeofmario/wiremap/internal/wiremap/config"
	apperrors "github.com/codeofmario/wiremap/internal/wiremap/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func statusOf(t *testing.T, err error) int {
	t.Helper()
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *AppError, got %T: %v", err, err)
	}
	return appErr.StatusCode
}

func TestWrapErrorMapsKubernetesErrors(t *testing.T) {
	pods := schema.GroupResource{Resource: "pods"}
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"not found", k8serrors.NewNotFound(pods, "api"), http.StatusNotFound},
		{"forbidden", k8serrors.NewForbidden(pods, "api", errors.New("rbac")), http.StatusForbidden},
		{"server error", k8serrors.NewInternalError(errors.New("etcd down")), http.StatusInternalServerError},
		{"plain error", errors.New("connection refused"), http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := WrapError(tt.err, "get pod")
			if got := statusOf(t, err); got != tt.want {
				t.Errorf("status = %d, want %d", got, tt.want)
			}
			if !strings.HasPrefix(err.Error(), "failed to get pod: ") {
				t.Errorf("message = %q", err.Error())
			}
		})
	}
}

func TestIsOptional(t *testing.T) {
	pods := schema.GroupResource{Resource: "pods"}
	tests := []struct {
		err  error
		want bool
	}{
		{k8serrors.NewNotFound(pods, "api"), true},
		{k8serrors.NewForbidden(pods, "api", errors.New("rbac")), true},
		{k8serrors.NewInternalError(errors.New("boom")), false},
		{errors.New("plain"), false},
	}
	for _, tt := range tests {
		if got := IsOptional(tt.err); got != tt.want {
			t.Errorf("IsOptional(%v) = %v, want %v", tt.err, got, tt.want)
		}
	}
}

var (
	deploymentGVK = schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
	nodeGVK       = schema.GroupVersionKind{Version: "v1", Kind: "Node"}
)

func staticMapper() *meta.DefaultRESTMapper {
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "apps", Version: "v1"}, {Version: "v1"}})
	mapper.Add(deploymentGVK, meta.RESTScopeNamespace)
	mapper.Add(nodeGVK, meta.RESTScopeRoot)
	return mapper
}

func TestResourceForResolvesKindsAndScope(t *testing.T) {
	c := &Cluster{Mapper: staticMapper()}

	deployment, err := c.ResourceFor("apps", "Deployment")
	if err != nil {
		t.Fatal(err)
	}
	if deployment.GVR != (schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}) || !deployment.Namespaced {
		t.Errorf("deployment mapping = %+v", deployment)
	}

	node, err := c.ResourceFor("", "Node")
	if err != nil {
		t.Fatal(err)
	}
	if node.GVR.Resource != "nodes" || node.Namespaced {
		t.Errorf("node mapping = %+v", node)
	}
}

func TestResourceForUnknownKindIsNotFound(t *testing.T) {
	c := &Cluster{Mapper: staticMapper()}

	_, err := c.ResourceFor("example.com", "Widget")
	if got := statusOf(t, err); got != http.StatusNotFound {
		t.Errorf("status = %d", got)
	}
	if c.Serves("example.com", "Widget") || !c.Serves("apps", "Deployment") {
		t.Error("Serves should reflect the mapper")
	}
}

// resettingMapper misses a kind until Reset is called, like a discovery cache
// that predates a newly installed CRD.
type resettingMapper struct {
	*meta.DefaultRESTMapper
	reset bool
}

func (m *resettingMapper) Reset() { m.reset = true }

func (m *resettingMapper) RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
	if !m.reset {
		return nil, &meta.NoKindMatchError{GroupKind: gk}
	}
	return m.DefaultRESTMapper.RESTMapping(gk, versions...)
}

func TestResourceForRefreshesDiscoveryOnMiss(t *testing.T) {
	mapper := &resettingMapper{DefaultRESTMapper: staticMapper()}
	c := &Cluster{Mapper: mapper}

	mapping, err := c.ResourceFor("apps", "Deployment")
	if err != nil {
		t.Fatal(err)
	}
	if !mapper.reset || mapping.GVR.Resource != "deployments" {
		t.Errorf("reset=%v mapping=%+v", mapper.reset, mapping)
	}
}

// failingMapper fails every lookup with an error other than a missing kind.
type failingMapper struct{ meta.RESTMapper }

func (failingMapper) RESTMapping(schema.GroupKind, ...string) (*meta.RESTMapping, error) {
	return nil, errors.New("discovery unavailable")
}

func TestResourceForDiscoveryFailureIsInternal(t *testing.T) {
	c := &Cluster{Mapper: failingMapper{}}
	_, err := c.ResourceFor("apps", "Deployment")
	if got := statusOf(t, err); got != http.StatusInternalServerError {
		t.Errorf("status = %d", got)
	}
}

func TestStaticClusterPool(t *testing.T) {
	pool := NewStaticClusterPool(&Cluster{Name: "prod"}, &Cluster{Name: "dev"})

	if pool.Connected() != 2 || !pool.IsConnected("prod") || pool.IsConnected("staging") {
		t.Errorf("connected=%d", pool.Connected())
	}
	if len(pool.Clusters()) != 2 || pool.Clusters()[0].Name != "prod" {
		t.Errorf("clusters = %+v", pool.Clusters())
	}
	c, err := pool.Get("dev")
	if err != nil || c.Name != "dev" {
		t.Errorf("Get(dev) = %+v, %v", c, err)
	}

	_, err = pool.Get("staging")
	if statusOf(t, err) != http.StatusNotFound || !strings.Contains(err.Error(), "not configured") {
		t.Errorf("unexpected error %v", err)
	}
}

// writeKubeconfig writes a kubeconfig with two contexts, "alpha" (current) and "beta".
func writeKubeconfig(t *testing.T, alphaServer, betaServer string) string {
	t.Helper()
	content := fmt.Sprintf(`apiVersion: v1
kind: Config
current-context: alpha
clusters:
- name: alpha
  cluster:
    server: %s
- name: beta
  cluster:
    server: %s
users:
- name: user
  user:
    token: secret
contexts:
- name: alpha
  context:
    cluster: alpha
    user: user
- name: beta
  context:
    cluster: beta
    user: user
`, alphaServer, betaServer)
	path := filepath.Join(t.TempDir(), "kubeconfig")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadRestConfigSelectsContext(t *testing.T) {
	path := writeKubeconfig(t, "https://alpha.example:6443", "https://beta.example:6443")

	restConfig, contextName, err := loadRestConfig(config.ClusterConfig{Kubeconfig: path})
	if err != nil {
		t.Fatal(err)
	}
	if contextName != "alpha" || restConfig.Host != "https://alpha.example:6443" || restConfig.BearerToken != "secret" {
		t.Errorf("current context: %q %q", contextName, restConfig.Host)
	}

	restConfig, contextName, err = loadRestConfig(config.ClusterConfig{Kubeconfig: path, Context: "beta"})
	if err != nil {
		t.Fatal(err)
	}
	if contextName != "beta" || restConfig.Host != "https://beta.example:6443" {
		t.Errorf("explicit context: %q %q", contextName, restConfig.Host)
	}
}

func TestLoadRestConfigErrors(t *testing.T) {
	path := writeKubeconfig(t, "https://alpha.example:6443", "https://beta.example:6443")

	if _, _, err := loadRestConfig(config.ClusterConfig{Kubeconfig: path, Context: "missing"}); err == nil {
		t.Error("unknown context should fail")
	}
	if _, _, err := loadRestConfig(config.ClusterConfig{Kubeconfig: filepath.Join(t.TempDir(), "absent")}); err == nil {
		t.Error("missing kubeconfig should fail")
	}

	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	_, contextName, err := loadRestConfig(config.ClusterConfig{InCluster: true})
	if err == nil || contextName != "in-cluster" {
		t.Errorf("in-cluster outside a pod: %q %v", contextName, err)
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}
	tests := map[string]string{
		"~/.kube/config":    home + "/.kube/config",
		"/etc/kube/config":  "/etc/kube/config",
		"relative/config":   "relative/config",
		"~":                 "~",
		"~other/kubeconfig": "~other/kubeconfig",
	}
	for in, want := range tests {
		if got := expandHome(in); got != want {
			t.Errorf("expandHome(%q) = %q, want %q", in, got, want)
		}
	}
}

// fakeAPIServer answers the /version request NewClusterPool pings on connect.
func fakeAPIServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/version" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"major":"1","minor":"31","gitVersion":"v1.31.0"}`)
	}))
	t.Cleanup(server.Close)
	return server
}

// closedAddress returns a local URL nothing listens on, so connecting fails fast.
func closedAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	listener.Close()
	return "http://" + addr
}

func TestNewClusterPoolConnectsAndReportsFailures(t *testing.T) {
	server := fakeAPIServer(t)
	path := writeKubeconfig(t, server.URL, closedAddress(t))

	pool := NewClusterPool(&config.Settings{Clusters: []config.ClusterConfig{
		{Kubeconfig: path}, // named after the current context
		{Name: "beta", Kubeconfig: path, Context: "beta"},      // unreachable
		{Name: "broken", Kubeconfig: path, Context: "missing"}, // invalid config
	}})

	if pool.Connected() != 1 || !pool.IsConnected("alpha") {
		t.Fatalf("connected=%d clusters=%+v", pool.Connected(), pool.Clusters())
	}
	alpha, err := pool.Get("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if alpha.RestConfig.QPS != 50 || alpha.RestConfig.Burst != 100 {
		t.Errorf("rate limits = %v/%v", alpha.RestConfig.QPS, alpha.RestConfig.Burst)
	}
	if alpha.Clientset == nil || alpha.Dynamic == nil || alpha.Metrics == nil || alpha.Mapper == nil {
		t.Errorf("clients not initialised: %+v", alpha)
	}

	if names := []string{pool.Clusters()[0].Name, pool.Clusters()[1].Name, pool.Clusters()[2].Name}; names[0] != "alpha" || names[1] != "beta" || names[2] != "broken" {
		t.Errorf("configured clusters = %v", names)
	}
	for _, name := range []string{"beta", "broken"} {
		_, err := pool.Get(name)
		if err == nil || !strings.Contains(err.Error(), "failed to connect") {
			t.Errorf("Get(%s) = %v", name, err)
		}
	}
}

func TestNewClusterPoolWithoutClusters(t *testing.T) {
	pool := NewClusterPool(&config.Settings{})
	if pool.Connected() != 0 || len(pool.Clusters()) != 0 {
		t.Errorf("expected empty pool, got %+v", pool.Clusters())
	}
}
