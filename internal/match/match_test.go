package match

import (
	"testing"

	"github.com/goldyfruit/elemental-node-mapper/internal/types"
)

func TestMatchOrderAndAmbiguity(t *testing.T) {
	nodes := []types.K8sNode{
		{Name: "node-1", UID: "1", MachineID: "MID-1", ProviderID: "prov-1", InternalIPs: []string{"10.0.0.1"}, ExternalIPs: []string{"1.1.1.1"}},
		{Name: "node-2", UID: "2", MachineID: "mid-1", ProviderID: "prov-2", InternalIPs: []string{"10.0.0.2"}, ExternalIPs: []string{"1.1.1.2"}},
		{Name: "node-3", UID: "3", ProviderID: "prov-3", InternalIPs: []string{"10.0.0.3"}, ExternalIPs: []string{"1.1.1.3"}},
		{Name: "node-4", UID: "4", InternalIPs: []string{"10.0.0.4"}, ExternalIPs: []string{"1.1.1.4"}},
		{Name: "host-5", UID: "5"},
		{Name: "node-6", UID: "6"},
	}

	hosts := []types.InventoryHost{
		{ID: "host-a", Hostname: "host-a", MachineID: "MID-1", ProviderID: "prov-3"},
		{ID: "host-b", Hostname: "host-b", ProviderID: "prov-3"},
		{ID: "host-c", Hostname: "host-c", IPs: []string{"10.0.0.4"}},
		{ID: "host-d", Hostname: "host-d", IPs: []string{"1.1.1.4"}},
		{ID: "host-e", Hostname: "host-5"},
		{ID: "host-f", Hostname: "host-f"},
	}

	result := Match(hosts, nodes)

	if len(result.Ambiguous) != 1 {
		t.Fatalf("expected 1 ambiguous match, got %d", len(result.Ambiguous))
	}
	if result.Ambiguous[0].Method != MethodMachineID {
		t.Fatalf("expected ambiguous match to use machine-id, got %s", result.Ambiguous[0].Method)
	}
	if len(result.Matches) != 4 {
		t.Fatalf("expected 4 definitive matches, got %d", len(result.Matches))
	}
	if len(result.UnmatchedHosts) != 1 {
		t.Fatalf("expected 1 unmatched host, got %d", len(result.UnmatchedHosts))
	}
	if len(result.UnmatchedNodes) != 1 {
		t.Fatalf("expected 1 unmatched node, got %d", len(result.UnmatchedNodes))
	}
	if result.UnmatchedNodes[0].Name != "node-6" {
		t.Fatalf("expected node-6 to be unmatched, got %s", result.UnmatchedNodes[0].Name)
	}
}

func TestMatchHostnameHexSuffix(t *testing.T) {
	nodes := []types.K8sNode{
		{Name: "smtl001-w-asus-1c32d8c7-cd47-2da5-19b7-bcfce773e4da-62c86a9b", UID: "node-1"},
	}
	hosts := []types.InventoryHost{
		{ID: "host-1", Hostname: "smtl001-w-asus-1c32d8c7-cd47-2da5-19b7-bcfce773e4da"},
	}

	result := Match(hosts, nodes)
	if len(result.Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(result.Matches))
	}
	if result.Matches[0].Method != MethodHostname {
		t.Fatalf("expected hostname match, got %s", result.Matches[0].Method)
	}
}

func TestMatchMachineNameFromNodeLabels(t *testing.T) {
	nodes := []types.K8sNode{
		{
			Name:   "node-1",
			UID:    "node-1",
			Labels: map[string]string{"cluster.x-k8s.io/machine": "shared-mtl-001-a9070xt-b26pf-v24bp"},
		},
	}
	hosts := []types.InventoryHost{
		{ID: "host-1", MachineName: "shared-mtl-001-a9070xt-b26pf-v24bp"},
	}

	result := Match(hosts, nodes)
	if len(result.Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(result.Matches))
	}
	if result.Matches[0].Method != MethodMachineName {
		t.Fatalf("expected machine-name match, got %s", result.Matches[0].Method)
	}
}
