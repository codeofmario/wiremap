package dto

// KubeGraphDto is one drill-down level of a cluster, ready to render on the canvas.
type KubeGraphDto struct {
	Level  string              `json:"level"`
	Nodes  []KubeGraphNodeDto  `json:"nodes"`
	Edges  []KubeGraphEdgeDto  `json:"edges"`
	Groups []KubeGraphGroupDto `json:"groups"`
}

type KubeGraphNodeDto struct {
	ID        string            `json:"id"`
	Kind      string            `json:"kind"`
	APIGroup  string            `json:"apiGroup,omitempty"`
	Name      string            `json:"name"`
	Namespace string            `json:"namespace,omitempty"`
	Status    string            `json:"status"`
	Summary   string            `json:"summary"`
	Drillable bool              `json:"drillable"`
	Group     string            `json:"group,omitempty"`
	Weight    int               `json:"weight"`
	Details   map[string]string `json:"details,omitempty"`
}

type KubeGraphEdgeDto struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Kind   string `json:"kind"`
}

type KubeGraphGroupDto struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type KubeClusterDto struct {
	Name      string `json:"name"`
	Connected bool   `json:"connected"`
}
