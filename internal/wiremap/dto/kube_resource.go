package dto

type KubeResourceDto struct {
	Kind      string         `json:"kind"`
	Group     string         `json:"group,omitempty"`
	Name      string         `json:"name"`
	Namespace string         `json:"namespace,omitempty"`
	YAML      string         `json:"yaml"`
	Events    []KubeEventDto `json:"events"`
	Actions   KubeActionsDto `json:"actions"`
}

// KubeActionsDto lists the actions the UI offers for an object and the state
// their controls start from. Nil pointers mean the action doesn't apply.
type KubeActionsDto struct {
	Edit          bool   `json:"edit"`
	Delete        bool   `json:"delete"`
	Restart       bool   `json:"restart"`
	Trigger       bool   `json:"trigger"`
	Replicas      *int32 `json:"replicas,omitempty"`
	Suspended     *bool  `json:"suspended,omitempty"`
	Unschedulable *bool  `json:"unschedulable,omitempty"`
}

type KubeEventDto struct {
	Type     string `json:"type"`
	Reason   string `json:"reason"`
	Message  string `json:"message"`
	Count    int32  `json:"count"`
	LastSeen string `json:"lastSeen"`
	Object   string `json:"object"`
}
