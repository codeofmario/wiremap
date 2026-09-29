package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const fileWithHostsAndClusters = `hosts:
  - name: remote
    url: ssh://user@remote
clusters:
  - name: prod
    kubeconfig: ~/.kube/prod
    context: prod-admin
  - name: in-cluster
    inCluster: true
`

func writeConfig(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, defaultConfigFile)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

var fileClusters = []ClusterConfig{
	{Name: "prod", Kubeconfig: "~/.kube/prod", Context: "prod-admin"},
	{Name: "in-cluster", InCluster: true},
}

func TestResolveClusters(t *testing.T) {
	file := &FileConfig{Clusters: fileClusters}
	tests := []struct {
		name       string
		file       *FileConfig
		kubeconfig string
		contexts   []string
		want       []ClusterConfig
	}{
		{
			"contexts flag makes one cluster per context", file, "/k/config", []string{"prod", "staging"},
			[]ClusterConfig{{Name: "prod", Kubeconfig: "/k/config", Context: "prod"}, {Name: "staging", Kubeconfig: "/k/config", Context: "staging"}},
		},
		{"contexts without kubeconfig use the default", nil, "", []string{"dev"}, []ClusterConfig{{Name: "dev", Context: "dev"}}},
		{"kubeconfig flag uses its current context", file, "/k/config", nil, []ClusterConfig{{Kubeconfig: "/k/config"}}},
		{"config file clusters", file, "", nil, fileClusters},
		{"kubernetes disabled without flags or file", nil, "", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveClusters(tt.file, tt.kubeconfig, tt.contexts); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestLoadConfigFile(t *testing.T) {
	path := writeConfig(t, t.TempDir(), fileWithHostsAndClusters)

	cfg, err := loadConfigFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.Clusters, fileClusters) || len(cfg.Hosts) != 1 || cfg.Hosts[0].Name != "remote" {
		t.Errorf("config = %+v", cfg)
	}

	if _, err := loadConfigFile(filepath.Join(t.TempDir(), "missing.yml")); err == nil {
		t.Error("missing file should fail")
	}
	bad := writeConfig(t, t.TempDir(), "hosts: [unclosed")
	if _, err := loadConfigFile(bad); err == nil {
		t.Error("invalid YAML should fail")
	}
}

func TestNewSettingsFromExplicitConfigFile(t *testing.T) {
	path := writeConfig(t, t.TempDir(), fileWithHostsAndClusters)

	s := NewSettings(Flags{Port: 7070, DevMode: true, ConfigFile: path, Hosts: []string{"tcp://ignored:2375"}})

	if s.Port != 7070 || !s.DevMode || s.ConfigFile != path {
		t.Errorf("flags not copied: %+v", s)
	}
	if len(s.Hosts) != 1 || s.Hosts[0].Name != "remote" {
		t.Errorf("explicit config file hosts should win over flags: %+v", s.Hosts)
	}
	if !reflect.DeepEqual(s.Clusters, fileClusters) {
		t.Errorf("clusters = %+v", s.Clusters)
	}
}

func TestNewSettingsKubeFlagsOverrideConfigFile(t *testing.T) {
	path := writeConfig(t, t.TempDir(), fileWithHostsAndClusters)

	s := NewSettings(Flags{ConfigFile: path, Kubeconfig: "/k/config", KubeContexts: []string{"dev"}})

	if want := []ClusterConfig{{Name: "dev", Kubeconfig: "/k/config", Context: "dev"}}; !reflect.DeepEqual(s.Clusters, want) {
		t.Errorf("clusters = %+v", s.Clusters)
	}
}

func TestNewSettingsReadsDefaultFileFromWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, fileWithHostsAndClusters)
	t.Chdir(dir)

	s := NewSettings(Flags{})

	if len(s.Hosts) != 1 || s.Hosts[0].Name != "remote" || !reflect.DeepEqual(s.Clusters, fileClusters) {
		t.Errorf("settings = %+v", s)
	}

	// Host flags win over the default file, but its clusters still apply
	s = NewSettings(Flags{Hosts: []string{"unix:///var/run/docker.sock", "tcp://prod:2375"}})
	want := []HostConfig{{Name: "local", URL: "unix:///var/run/docker.sock"}, {Name: "host-2", URL: "tcp://prod:2375"}}
	if !reflect.DeepEqual(s.Hosts, want) || !reflect.DeepEqual(s.Clusters, fileClusters) {
		t.Errorf("hosts = %+v clusters = %+v", s.Hosts, s.Clusters)
	}
}

func TestNewSettingsDefaultsWithoutAnyConfig(t *testing.T) {
	t.Chdir(t.TempDir())

	s := NewSettings(Flags{})

	if want := []HostConfig{{Name: "local", URL: "unix:///var/run/docker.sock"}}; !reflect.DeepEqual(s.Hosts, want) {
		t.Errorf("hosts = %+v", s.Hosts)
	}
	if s.Clusters != nil {
		t.Errorf("kubernetes should be disabled, got %+v", s.Clusters)
	}
}

func TestNewSettingsIgnoresUnreadableConfigFile(t *testing.T) {
	t.Chdir(t.TempDir())

	s := NewSettings(Flags{ConfigFile: filepath.Join(t.TempDir(), "missing.yml")})

	if len(s.Hosts) != 1 || s.Hosts[0].Name != "local" || s.Clusters != nil {
		t.Errorf("settings = %+v", s)
	}
}

func TestNewSettingsSkipsInvalidDefaultFile(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "hosts: [unclosed")
	t.Chdir(dir)

	s := NewSettings(Flags{})

	if len(s.Hosts) != 1 || s.Hosts[0].Name != "local" || s.Clusters != nil {
		t.Errorf("settings = %+v", s)
	}
}

func TestHostName(t *testing.T) {
	tests := []struct {
		url   string
		index int
		want  string
	}{
		{"unix:///var/run/docker.sock", 3, "local"},
		{"", 0, "local"},
		{"tcp://prod:2375", 0, "host-1"},
		{"ssh://user@remote", 4, "host-5"},
	}
	for _, tt := range tests {
		if got := hostName(tt.url, tt.index); got != tt.want {
			t.Errorf("hostName(%q, %d) = %q, want %q", tt.url, tt.index, got, tt.want)
		}
	}
}
