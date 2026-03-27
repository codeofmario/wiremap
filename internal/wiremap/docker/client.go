package docker

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
	"os/exec"
	"strings"
	"sync"

	"github.com/codeofmario/wiremap/internal/wiremap/config"
	"github.com/docker/docker/client"
)

type ClientPool struct {
	clients map[string]*client.Client
	hosts   []config.HostConfig
	mu      sync.RWMutex
}

func NewClientPool(settings *config.Settings) (*ClientPool, error) {
	pool := &ClientPool{
		clients: make(map[string]*client.Client),
		hosts:   settings.Hosts,
	}

	for _, host := range settings.Hosts {
		c, err := createClient(host)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to create client for host %s: %v\n", host.Name, err)
			continue
		}
		pool.clients[host.Name] = c
	}

	if len(pool.clients) == 0 {
		return nil, fmt.Errorf("no Docker hosts could be connected")
	}

	return pool, nil
}

func (p *ClientPool) Get(hostID string) (*client.Client, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if hostID == "" && len(p.hosts) > 0 {
		hostID = p.hosts[0].Name
	}

	c, ok := p.clients[hostID]
	if !ok {
		return nil, fmt.Errorf("host %q not found or not connected", hostID)
	}
	return c, nil
}

func (p *ClientPool) Hosts() []config.HostConfig {
	return p.hosts
}

func createClient(host config.HostConfig) (*client.Client, error) {
	opts := []client.Opt{
		client.WithAPIVersionNegotiation(),
	}

	if strings.HasPrefix(host.URL, "ssh://") {
		return createSSHClient(host.URL)
	} else if host.URL != "" {
		opts = append(opts, client.WithHost(host.URL))
	}

	if host.TLS != nil {
		httpClient, err := tlsHTTPClient(host.TLS)
		if err != nil {
			return nil, err
		}
		opts = append(opts, client.WithHTTPClient(httpClient))
	}

	return client.NewClientWithOpts(opts...)
}

func createSSHClient(sshURL string) (*client.Client, error) {
	sshTarget := strings.TrimPrefix(sshURL, "ssh://")

	helper := &sshConnHelper{target: sshTarget}

	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: helper.dial,
		},
	}

	return client.NewClientWithOpts(
		client.WithHTTPClient(httpClient),
		client.WithHost("http://localhost:2375"),
		client.WithAPIVersionNegotiation(),
		client.WithDialContext(helper.dial),
	)
}

type sshConnHelper struct {
	target string
}

func (h *sshConnHelper) dial(ctx context.Context, _, _ string) (net.Conn, error) {
	return sshDialStdio(ctx, h.target)
}

func sshHTTPClient(sshURL string) *http.Client {
	// ssh://user@host -> user@host
	sshTarget := strings.TrimPrefix(sshURL, "ssh://")

	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return sshDialStdio(ctx, sshTarget)
			},
		},
	}
}

func sshDialStdio(ctx context.Context, sshTarget string) (net.Conn, error) {
	// Try docker system dial-stdio first, fall back to socat
	cmd := exec.CommandContext(ctx,
		"ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "ConnectTimeout=10",
		sshTarget,
		"docker", "system", "dial-stdio",
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("ssh stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("ssh stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("ssh start: %w", err)
	}

	return &sshConn{
		cmd:    cmd,
		reader: stdout,
		writer: stdin,
	}, nil
}

type sshConn struct {
	cmd    *exec.Cmd
	reader interface{ Read([]byte) (int, error) }
	writer interface {
		Write([]byte) (int, error)
		Close() error
	}
}

func (c *sshConn) Read(b []byte) (int, error)  { return c.reader.Read(b) }
func (c *sshConn) Write(b []byte) (int, error) { return c.writer.Write(b) }
func (c *sshConn) Close() error {
	c.writer.Close()
	return c.cmd.Process.Kill()
}
func (c *sshConn) LocalAddr() net.Addr                { return &net.UnixAddr{Name: "ssh", Net: "ssh"} }
func (c *sshConn) RemoteAddr() net.Addr               { return &net.UnixAddr{Name: "ssh", Net: "ssh"} }
func (c *sshConn) SetDeadline(_ time.Time) error      { return nil }
func (c *sshConn) SetReadDeadline(_ time.Time) error  { return nil }
func (c *sshConn) SetWriteDeadline(_ time.Time) error { return nil }

func tlsHTTPClient(cfg *config.TLSConfig) (*http.Client, error) {
	cert, err := tls.LoadX509KeyPair(cfg.CertPath, cfg.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS cert/key: %w", err)
	}

	caCert, err := os.ReadFile(cfg.CAPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load CA cert: %w", err)
	}

	caPool := x509.NewCertPool()
	caPool.AppendCertsFromPEM(caCert)

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caPool,
	}

	return &http.Client{
		Transport: &http.Transport{TLSClientConfig: tlsConfig},
	}, nil
}
