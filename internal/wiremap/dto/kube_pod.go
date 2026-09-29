package dto

type KubePodDto struct {
	Name       string             `json:"name"`
	Namespace  string             `json:"namespace"`
	Node       string             `json:"node"`
	Phase      string             `json:"phase"`
	Status     string             `json:"status"`
	PodIP      string             `json:"podIp"`
	HostIP     string             `json:"hostIp"`
	StartTime  string             `json:"startTime,omitempty"`
	Labels     map[string]string  `json:"labels"`
	Containers []KubeContainerDto `json:"containers"`
}

type KubeContainerDto struct {
	Name         string `json:"name"`
	Image        string `json:"image"`
	Ready        bool   `json:"ready"`
	RestartCount int32  `json:"restartCount"`
	State        string `json:"state"`
	Reason       string `json:"reason,omitempty"`
	Init         bool   `json:"init"`
}
