package service

// API groups of the built-in and Gateway API kinds the canvas draws; core kinds are "".
var kindGroups = map[string]string{
	"Deployment":              "apps",
	"StatefulSet":             "apps",
	"DaemonSet":               "apps",
	"ReplicaSet":              "apps",
	"Job":                     "batch",
	"CronJob":                 "batch",
	"Ingress":                 "networking.k8s.io",
	"NetworkPolicy":           "networking.k8s.io",
	"HorizontalPodAutoscaler": "autoscaling",
	"PodDisruptionBudget":     "policy",
	"Role":                    "rbac.authorization.k8s.io",
	"RoleBinding":             "rbac.authorization.k8s.io",
	"ClusterRole":             "rbac.authorization.k8s.io",
	"ClusterRoleBinding":      "rbac.authorization.k8s.io",
	"StorageClass":            "storage.k8s.io",
	"Gateway":                 gatewayGroup,
	"HTTPRoute":               gatewayGroup,
	"GRPCRoute":               gatewayGroup,
	"TLSRoute":                gatewayGroup,
	"TCPRoute":                gatewayGroup,
	"UDPRoute":                gatewayGroup,
}

const gatewayGroup = "gateway.networking.k8s.io"
