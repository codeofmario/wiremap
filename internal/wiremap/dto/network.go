package dto

type NetworkDto struct {
	ID         string                        `json:"id"`
	Name       string                        `json:"name"`
	Driver     string                        `json:"driver"`
	Scope      string                        `json:"scope"`
	Subnet     string                        `json:"subnet,omitempty"`
	Gateway    string                        `json:"gateway,omitempty"`
	Containers map[string]NetworkContainerDto `json:"containers"`
}

type NetworkContainerDto struct {
	Name        string `json:"name"`
	IPv4Address string `json:"ipv4Address"`
	MacAddress  string `json:"macAddress"`
}
