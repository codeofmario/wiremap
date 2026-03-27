package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type HostConfig struct {
	Name string    `yaml:"name"`
	URL  string    `yaml:"url"`
	TLS  *TLSConfig `yaml:"tls,omitempty"`
}

type TLSConfig struct {
	CertPath string `yaml:"cert"`
	KeyPath  string `yaml:"key"`
	CAPath   string `yaml:"ca"`
}

type FileConfig struct {
	Hosts []HostConfig `yaml:"hosts"`
}

type Settings struct {
	Port       int
	DevMode    bool
	ConfigFile string
	HostFlags  []string
	Hosts      []HostConfig
}

func NewSettings(port int, devMode bool, configFile string, hostFlags []string) *Settings {
	s := &Settings{
		Port:       port,
		DevMode:    devMode,
		ConfigFile: configFile,
		HostFlags:  hostFlags,
	}

	s.Hosts = s.resolveHosts()
	return s
}

func (s *Settings) resolveHosts() []HostConfig {
	// Try config file first
	if s.ConfigFile != "" {
		hosts, err := loadConfigFile(s.ConfigFile)
		if err == nil && len(hosts) > 0 {
			return hosts
		}
		fmt.Fprintf(os.Stderr, "warning: failed to load config file %s: %v\n", s.ConfigFile, err)
	}

	// Try CLI flags
	if len(s.HostFlags) > 0 {
		hosts := make([]HostConfig, 0, len(s.HostFlags))
		for i, h := range s.HostFlags {
			name := hostName(h, i)
			hosts = append(hosts, HostConfig{Name: name, URL: h})
		}
		return hosts
	}

	// Try default config file
	if _, err := os.Stat("wiremap.yml"); err == nil {
		hosts, err := loadConfigFile("wiremap.yml")
		if err == nil && len(hosts) > 0 {
			return hosts
		}
	}

	// Default: local docker socket
	return []HostConfig{
		{Name: "local", URL: "unix:///var/run/docker.sock"},
	}
}

func loadConfigFile(path string) ([]HostConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg FileConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return cfg.Hosts, nil
}

func hostName(url string, index int) string {
	if url == "unix:///var/run/docker.sock" || url == "" {
		return "local"
	}
	return fmt.Sprintf("host-%d", index+1)
}
