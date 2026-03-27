package dto

type StatsDto struct {
	CPUPercent    float64 `json:"cpuPercent"`
	MemoryUsage   uint64  `json:"memoryUsage"`
	MemoryLimit   uint64  `json:"memoryLimit"`
	MemoryPercent float64 `json:"memoryPercent"`
	NetworkRx     uint64  `json:"networkRx"`
	NetworkTx     uint64  `json:"networkTx"`
	Timestamp     string  `json:"timestamp"`
}

type LogEntryDto struct {
	ContainerID string `json:"containerId"`
	Type        string `json:"type"`
	Text        string `json:"text"`
}
