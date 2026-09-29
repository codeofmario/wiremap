package dto

// KubeKindDto is one listable resource type served by the cluster, built-in or CRD.
type KubeKindDto struct {
	Group      string `json:"group"`
	Version    string `json:"version"`
	Resource   string `json:"resource"`
	Kind       string `json:"kind"`
	Namespaced bool   `json:"namespaced"`
	// Aliases such as "po" or "deploy", for the UI's command bar
	ShortNames []string `json:"shortNames,omitempty"`
}

type KubeObjectListDto struct {
	Items []KubeObjectDto `json:"items"`
	// Token for the next page; empty on the last one
	Continue string `json:"continue,omitempty"`
}

type KubeObjectDto struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	Kind      string `json:"kind"`
	Group     string `json:"group,omitempty"`
	Status    string `json:"status"`
	Summary   string `json:"summary"`
	Created   string `json:"created"`
	// The topology can open the object to show what it contains
	Drillable bool `json:"drillable"`
}
