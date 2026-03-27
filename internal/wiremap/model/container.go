package model

type ContainerNetwork struct {
	IPAddress  string   `json:"ipAddress"`
	Gateway    string   `json:"gateway"`
	MacAddress string   `json:"macAddress"`
	NetworkID  string   `json:"networkId"`
	Aliases    []string `json:"aliases"`
}

type PortMapping struct {
	IP          string `json:"ip"`
	PrivatePort uint16 `json:"privatePort"`
	PublicPort  uint16 `json:"publicPort"`
	Type        string `json:"type"`
}

type MountInfo struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Mode        string `json:"mode"`
	RW          bool   `json:"rw"`
}
