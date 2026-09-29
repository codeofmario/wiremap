package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const defaultConfigFile = "wiremap.yml"

type HostConfig struct {
	Name string     `yaml:"name"`
	URL  string     `yaml:"url"`
	TLS  *TLSConfig `yaml:"tls,omitempty"`
}

type TLSConfig struct {
	CertPath string `yaml:"cert"`
	KeyPath  string `yaml:"key"`
	CAPath   string `yaml:"ca"`
}

type ClusterConfig struct {
	Name       string `yaml:"name"`
	Kubeconfig string `yaml:"kubeconfig,omitempty"`
	Context    string `yaml:"context,omitempty"`
	InCluster  bool   `yaml:"inCluster,omitempty"`
}

type FileConfig struct {
	Hosts    []HostConfig    `yaml:"hosts"`
	Clusters []ClusterConfig `yaml:"clusters"`
}

// Flags holds the raw CLI flag values passed to the application.
type Flags struct {
	Port         int
	DevMode      bool
	ConfigFile   string
	Hosts        []string
	Kubeconfig   string
	KubeContexts []string
}

type Settings struct {
	Port       int
	DevMode    bool
	ConfigFile string
	HostFlags  []string
	Hosts      []HostConfig
	Clusters   []ClusterConfig
}

func NewSettings(flags Flags) *Settings {
	s := &Settings{
		Port:       flags.Port,
		DevMode:    flags.DevMode,
		ConfigFile: flags.ConfigFile,
		HostFlags:  flags.Hosts,
	}

	file := s.loadFile()
	s.Hosts = s.resolveHosts(file)
	s.Clusters = resolveClusters(file, flags.Kubeconfig, flags.KubeContexts)
	return s
}

// loadFile reads the explicit --config file, or wiremap.yml from the working directory.
func (s *Settings) loadFile() *FileConfig {
	if s.ConfigFile != "" {
		cfg, err := loadConfigFile(s.ConfigFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to load config file %s: %v\n", s.ConfigFile, err)
		}
		return cfg
	}

	if _, err := os.Stat(defaultConfigFile); err == nil {
		cfg, err := loadConfigFile(defaultConfigFile)
		if err == nil {
			return cfg
		}
	}
	return nil
}

func (s *Settings) resolveHosts(file *FileConfig) []HostConfig {
	// Explicit config file wins
	if s.ConfigFile != "" && file != nil && len(file.Hosts) > 0 {
		return file.Hosts
	}

	// Then CLI flags
	if len(s.HostFlags) > 0 {
		hosts := make([]HostConfig, 0, len(s.HostFlags))
		for i, h := range s.HostFlags {
			name := hostName(h, i)
			hosts = append(hosts, HostConfig{Name: name, URL: h})
		}
		return hosts
	}

	// Then default config file
	if s.ConfigFile == "" && file != nil && len(file.Hosts) > 0 {
		return file.Hosts
	}

	// Default: local docker socket
	return []HostConfig{
		{Name: "local", URL: "unix:///var/run/docker.sock"},
	}
}

// resolveClusters returns the Kubernetes clusters to connect to. CLI flags take
// precedence over the config file; with neither, Kubernetes support stays disabled.
func resolveClusters(file *FileConfig, kubeconfig string, contexts []string) []ClusterConfig {
	if len(contexts) > 0 {
		clusters := make([]ClusterConfig, 0, len(contexts))
		for _, ctx := range contexts {
			clusters = append(clusters, ClusterConfig{Name: ctx, Kubeconfig: kubeconfig, Context: ctx})
		}
		return clusters
	}

	if kubeconfig != "" {
		// Name is filled in with the kubeconfig's current context on connect
		return []ClusterConfig{{Kubeconfig: kubeconfig}}
	}

	if file != nil {
		return file.Clusters
	}
	return nil
}

func loadConfigFile(path string) (*FileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg FileConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func hostName(url string, index int) string {
	if url == "unix:///var/run/docker.sock" || url == "" {
		return "local"
	}
	return fmt.Sprintf("host-%d", index+1)
}
