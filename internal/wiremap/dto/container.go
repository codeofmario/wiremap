package dto

type ContainerListItem struct {
	ID       string            `json:"id"`
	Names    []string          `json:"names"`
	Image    string            `json:"image"`
	State    string            `json:"state"`
	Status   string            `json:"status"`
	Created  int64             `json:"created"`
	Ports    []PortMappingDto  `json:"ports"`
	Labels   map[string]string `json:"labels"`
	Networks map[string]NetworkAttachment `json:"networks"`
}

type PortMappingDto struct {
	IP          string `json:"ip,omitempty"`
	PrivatePort uint16 `json:"privatePort"`
	PublicPort  uint16 `json:"publicPort,omitempty"`
	Type        string `json:"type"`
}

type NetworkAttachment struct {
	IPAddress  string   `json:"ipAddress"`
	Gateway    string   `json:"gateway"`
	MacAddress string   `json:"macAddress"`
	NetworkID  string   `json:"networkId"`
	Aliases    []string `json:"aliases"`
}

type ContainerInspectDto struct {
	ID              string                      `json:"id"`
	Name            string                      `json:"name"`
	State           ContainerStateDto           `json:"state"`
	Image           string                      `json:"image"`
	Command         string                      `json:"command"`
	Entrypoint      []string                    `json:"entrypoint"`
	Env             []string                    `json:"env"`
	Labels          map[string]string           `json:"labels"`
	RestartPolicy   string                      `json:"restartPolicy"`
	Mounts          []MountDto                  `json:"mounts"`
	PortBindings    map[string][]HostBinding    `json:"portBindings"`
	Networks        map[string]NetworkAttachment `json:"networks"`
	Created         string                      `json:"created"`
}

type ContainerStateDto struct {
	Status     string `json:"status"`
	Running    bool   `json:"running"`
	Paused     bool   `json:"paused"`
	Restarting bool   `json:"restarting"`
	StartedAt  string `json:"startedAt"`
	FinishedAt string `json:"finishedAt"`
	Health     string `json:"health,omitempty"`
}

type MountDto struct {
	Type        string `json:"type"`
	Name        string `json:"name,omitempty"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Mode        string `json:"mode"`
	RW          bool   `json:"rw"`
}

type HostBinding struct {
	HostIP   string `json:"hostIp"`
	HostPort string `json:"hostPort"`
}
