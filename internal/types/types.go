package types

// K8sNode is a normalized view of a Kubernetes node.
type K8sNode struct {
	Name        string
	UID         string
	Labels      map[string]string
	ProviderID  string
	MachineID   string
	MachineName string
	InternalIPs []string
	ExternalIPs []string
	Annotations map[string]string
}

// InventoryHost is a normalized view of a Rancher Elemental inventory host.
type InventoryHost struct {
	ID          string
	UID         string
	Namespace   string
	MachineName string
	Hostname    string
	MachineID   string
	SystemUUID  string
	ProviderID  string
	IPs         []string
	Labels      map[string]string
	Metadata    map[string]string
}
