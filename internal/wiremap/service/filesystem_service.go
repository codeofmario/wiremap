package service

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/codeofmario/wiremap/internal/wiremap/docker"
	"github.com/codeofmario/wiremap/internal/wiremap/dto"
	apperrors "github.com/codeofmario/wiremap/internal/wiremap/errors"
	"github.com/docker/docker/api/types"
)

type FilesystemService interface {
	ListDir(ctx context.Context, hostID string, containerID string, path string) ([]dto.FileEntry, error)
	ReadFile(ctx context.Context, hostID string, containerID string, path string) (*dto.FileContent, error)
	WriteFile(ctx context.Context, hostID string, containerID string, path string, content string) error
}

type filesystemService struct {
	pool *docker.ClientPool
}

func NewFilesystemService(pool *docker.ClientPool) FilesystemService {
	return &filesystemService{pool: pool}
}

func (s *filesystemService) execCmd(ctx context.Context, hostID string, containerID string, cmd []string) ([]byte, error) {
	c, err := s.pool.Get(hostID)
	if err != nil {
		return nil, err
	}

	exec, err := c.ContainerExecCreate(ctx, containerID, types.ExecConfig{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return nil, err
	}

	resp, err := c.ContainerExecAttach(ctx, exec.ID, types.ExecStartCheck{})
	if err != nil {
		return nil, err
	}
	defer resp.Close()

	var buf bytes.Buffer
	buf.ReadFrom(resp.Reader)
	return stripDockerHeaders(buf.Bytes()), nil
}

func (s *filesystemService) ListDir(ctx context.Context, hostID string, containerID string, path string) ([]dto.FileEntry, error) {
	if path == "" {
		path = "/"
	}

	output, err := s.execCmd(ctx, hostID, containerID, []string{"ls", "-la", path})
	if err != nil {
		return nil, apperrors.Internal(fmt.Sprintf("failed to list directory: %s", err))
	}

	entries := make([]dto.FileEntry, 0)
	for _, line := range strings.Split(string(output), "\n") {
		clean := strings.TrimSpace(line)
		if clean == "" || strings.HasPrefix(clean, "total") {
			continue
		}

		entry := parseLsLine(clean, path)
		if entry != nil && entry.Name != "." && entry.Name != ".." {
			entries = append(entries, *entry)
		}
	}

	return entries, nil
}

func parseLsLine(line string, basePath string) *dto.FileEntry {
	// ls -la output: perms links owner group size month day time name
	// BusyBox:       perms links owner group size month day time name
	// Minimum: perms + name with stuff in between
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return nil
	}

	perms := fields[0]
	if len(perms) < 2 || (perms[0] != '-' && perms[0] != 'd' && perms[0] != 'l' && perms[0] != 'c' && perms[0] != 'b' && perms[0] != 's' && perms[0] != 'p') {
		return nil
	}

	// Name is always the last field (may include spaces via symlink arrows)
	name := fields[len(fields)-1]

	// Find the size field — it's the first numeric field after the group (field 3+)
	var size int64
	for i := 3; i < len(fields)-1; i++ {
		if n, err := strconv.ParseInt(fields[i], 10, 64); err == nil {
			size = n
			break
		}
	}

	// Handle symlinks: last fields might be "name -> target"
	for i := len(fields) - 2; i >= 4; i-- {
		if fields[i] == "->" {
			name = fields[i-1]
			break
		}
	}

	// Strip directory prefix from name (ls on a file returns full path)
	if strings.Contains(name, "/") {
		parts := strings.Split(name, "/")
		name = parts[len(parts)-1]
	}

	fullPath := basePath
	if !strings.HasSuffix(fullPath, "/") {
		fullPath += "/"
	}
	fullPath += name

	return &dto.FileEntry{
		Name:        name,
		Path:        fullPath,
		IsDir:       perms[0] == 'd',
		Size:        size,
		Permissions: perms,
	}
}

func (s *filesystemService) ReadFile(ctx context.Context, hostID string, containerID string, path string) (*dto.FileContent, error) {
	output, err := s.execCmd(ctx, hostID, containerID, []string{"head", "-c", "524288", path})
	if err != nil {
		return nil, apperrors.Internal(fmt.Sprintf("failed to read file: %s", err))
	}

	isBinary := !utf8.Valid(output)

	return &dto.FileContent{
		Path:    path,
		Content: string(output),
		Binary:  isBinary,
		Size:    int64(len(output)),
	}, nil
}

func (s *filesystemService) WriteFile(ctx context.Context, hostID string, containerID string, path string, content string) error {
	c, err := s.pool.Get(hostID)
	if err != nil {
		return err
	}

	exec, err := c.ContainerExecCreate(ctx, containerID, types.ExecConfig{
		Cmd:         []string{"tee", path},
		AttachStdin: true,
	})
	if err != nil {
		return apperrors.Internal(fmt.Sprintf("failed to create exec for write: %s", err))
	}

	resp, err := c.ContainerExecAttach(ctx, exec.ID, types.ExecStartCheck{})
	if err != nil {
		return apperrors.Internal(fmt.Sprintf("failed to attach exec for write: %s", err))
	}
	defer resp.Close()

	_, err = resp.Conn.Write([]byte(content))
	if err != nil {
		return apperrors.Internal(fmt.Sprintf("failed to write content: %s", err))
	}
	resp.CloseWrite()

	return nil
}

func stripDockerHeaders(data []byte) []byte {
	var result []byte
	for len(data) >= 8 {
		streamType := data[0]
		if streamType > 2 {
			// Not a docker stream header
			result = append(result, data...)
			break
		}
		size := int(data[4])<<24 | int(data[5])<<16 | int(data[6])<<8 | int(data[7])
		data = data[8:]
		if size > len(data) {
			size = len(data)
		}
		result = append(result, data[:size]...)
		data = data[size:]
	}
	if len(data) > 0 && len(result) == 0 {
		return data
	}
	return result
}
