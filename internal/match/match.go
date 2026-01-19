package match

import (
	"fmt"
	"net"
	"sort"
	"strings"

	"github.com/goldyfruit/elemental-node-mapper/internal/types"
)

type Method string

const (
	MethodMachineID   Method = "machine-id"
	MethodProviderID  Method = "provider-id"
	MethodInternalIP  Method = "internal-ip"
	MethodExternalIP  Method = "external-ip"
	MethodMachineName Method = "machine-name"
	MethodHostname    Method = "hostname"
)

var methodConfidence = map[Method]float64{
	MethodMachineID:   0.98,
	MethodProviderID:  0.95,
	MethodInternalIP:  0.9,
	MethodExternalIP:  0.85,
	MethodMachineName: 0.75,
	MethodHostname:    0.7,
}

type NodeMatch struct {
	Node        types.K8sNode
	Method      Method
	Confidence  float64
	Explanation string
}

type HostMatch struct {
	Host       types.InventoryHost
	Candidates []NodeMatch
	Method     Method
	Confidence float64
}

type Result struct {
	Matches        []HostMatch
	Ambiguous      []HostMatch
	UnmatchedHosts []types.InventoryHost
	UnmatchedNodes []types.K8sNode
}

type nodeIndex struct {
	byMachineID   map[string][]types.K8sNode
	byProviderID  map[string][]types.K8sNode
	byInternalIP  map[string][]types.K8sNode
	byExternalIP  map[string][]types.K8sNode
	byMachineName map[string][]types.K8sNode
	byHostname    map[string][]types.K8sNode
}

func Match(hosts []types.InventoryHost, nodes []types.K8sNode) Result {
	index := buildIndex(nodes)
	result := Result{}
	nodeSeen := make(map[string]struct{})

	for _, host := range hosts {
		if matches, ok := matchByMachineID(host, index); ok {
			result.addMatch(host, matches, nodeSeen)
			continue
		}
		if matches, ok := matchByProviderID(host, index); ok {
			result.addMatch(host, matches, nodeSeen)
			continue
		}
		if matches, ok := matchByInternalIP(host, index); ok {
			result.addMatch(host, matches, nodeSeen)
			continue
		}
		if matches, ok := matchByExternalIP(host, index); ok {
			result.addMatch(host, matches, nodeSeen)
			continue
		}
		if matches, ok := matchByHostname(host, index); ok {
			result.addMatch(host, matches, nodeSeen)
			continue
		}
		result.UnmatchedHosts = append(result.UnmatchedHosts, host)
	}

	result.UnmatchedNodes = append(result.UnmatchedNodes, collectUnmatched(nodes, nodeSeen)...)
	return result
}

func (r *Result) addMatch(host types.InventoryHost, matches []NodeMatch, nodeSeen map[string]struct{}) {
	if len(matches) == 1 {
		match := HostMatch{
			Host:       host,
			Candidates: matches,
			Method:     matches[0].Method,
			Confidence: matches[0].Confidence,
		}
		r.Matches = append(r.Matches, match)
	} else {
		match := HostMatch{
			Host:       host,
			Candidates: matches,
			Method:     matches[0].Method,
			Confidence: matches[0].Confidence,
		}
		r.Ambiguous = append(r.Ambiguous, match)
	}

	for _, candidate := range matches {
		nodeSeen[nodeKey(candidate.Node)] = struct{}{}
	}
}

func collectUnmatched(nodes []types.K8sNode, nodeSeen map[string]struct{}) []types.K8sNode {
	var unmatched []types.K8sNode
	for _, node := range nodes {
		if _, ok := nodeSeen[nodeKey(node)]; !ok {
			unmatched = append(unmatched, node)
		}
	}
	return unmatched
}

func matchByMachineID(host types.InventoryHost, index nodeIndex) ([]NodeMatch, bool) {
	keys := normalizedIDs(host.MachineID, host.SystemUUID)
	return matchByKeys(keys, index.byMachineID, MethodMachineID, "machine-id")
}

func matchByProviderID(host types.InventoryHost, index nodeIndex) ([]NodeMatch, bool) {
	keys := normalizedIDs(host.ProviderID)
	return matchByKeys(keys, index.byProviderID, MethodProviderID, "provider-id")
}

func matchByInternalIP(host types.InventoryHost, index nodeIndex) ([]NodeMatch, bool) {
	keys := normalizedIPs(host.IPs)
	return matchByKeys(keys, index.byInternalIP, MethodInternalIP, "internal-ip")
}

func matchByExternalIP(host types.InventoryHost, index nodeIndex) ([]NodeMatch, bool) {
	keys := normalizedIPs(host.IPs)
	return matchByKeys(keys, index.byExternalIP, MethodExternalIP, "external-ip")
}

func matchByHostname(host types.InventoryHost, index nodeIndex) ([]NodeMatch, bool) {
	if matches, ok := matchByMachineName(host, index); ok {
		return matches, true
	}
	keys := normalizedHostnames(host.Hostname)
	if len(keys) == 0 {
		return nil, false
	}
	return matchByKeys(keys, index.byHostname, MethodHostname, "hostname")
}

func matchByMachineName(host types.InventoryHost, index nodeIndex) ([]NodeMatch, bool) {
	keys := normalizedHostnames(host.MachineName)
	return matchByKeys(keys, index.byMachineName, MethodMachineName, "machine-name")
}

func matchByKeys(keys []string, index map[string][]types.K8sNode, method Method, reason string) ([]NodeMatch, bool) {
	if len(keys) == 0 {
		return nil, false
	}
	candidates := make(map[string]NodeMatch)
	for _, key := range keys {
		for _, node := range index[key] {
			keyID := nodeKey(node)
			if _, ok := candidates[keyID]; ok {
				continue
			}
			candidates[keyID] = NodeMatch{
				Node:        node,
				Method:      method,
				Confidence:  methodConfidence[method],
				Explanation: fmt.Sprintf("%s=%s", reason, key),
			}
		}
	}

	if len(candidates) == 0 {
		return nil, false
	}

	matches := make([]NodeMatch, 0, len(candidates))
	for _, match := range candidates {
		matches = append(matches, match)
	}
	stableMatchSort(matches)
	return matches, true
}

func buildIndex(nodes []types.K8sNode) nodeIndex {
	idx := nodeIndex{
		byMachineID:   make(map[string][]types.K8sNode),
		byProviderID:  make(map[string][]types.K8sNode),
		byInternalIP:  make(map[string][]types.K8sNode),
		byExternalIP:  make(map[string][]types.K8sNode),
		byMachineName: make(map[string][]types.K8sNode),
		byHostname:    make(map[string][]types.K8sNode),
	}

	for _, node := range nodes {
		if key := normalizeID(node.MachineID); key != "" {
			idx.byMachineID[key] = append(idx.byMachineID[key], node)
		}
		if key := normalizeID(node.ProviderID); key != "" {
			idx.byProviderID[key] = append(idx.byProviderID[key], node)
		}
		for _, ip := range normalizedIPs(node.InternalIPs) {
			idx.byInternalIP[ip] = append(idx.byInternalIP[ip], node)
		}
		for _, ip := range normalizedIPs(node.ExternalIPs) {
			idx.byExternalIP[ip] = append(idx.byExternalIP[ip], node)
		}
		for _, key := range nodeMachineNames(node) {
			idx.byMachineName[key] = append(idx.byMachineName[key], node)
		}
		for _, key := range normalizedHostnames(node.Name) {
			idx.byHostname[key] = append(idx.byHostname[key], node)
		}
	}
	return idx
}

func normalizedIDs(values ...string) []string {
	var keys []string
	for _, value := range values {
		key := normalizeID(value)
		if key != "" {
			keys = append(keys, key)
		}
	}
	return uniqueSorted(keys)
}

func normalizeID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return strings.ToLower(value)
}

func normalizeHostname(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.TrimSuffix(value, ".")
	return strings.ToLower(value)
}

func normalizedHostnames(value string) []string {
	full := normalizeHostname(value)
	if full == "" {
		return nil
	}
	short := full
	if idx := strings.Index(short, "."); idx > 0 {
		short = short[:idx]
	}

	variants := []string{full}
	if short != full {
		variants = append(variants, short)
	}
	if trimmed := trimHexSuffix(full); trimmed != full {
		variants = append(variants, trimmed)
		if trimmedShort := trimHexSuffix(short); trimmedShort != short {
			variants = append(variants, trimmedShort)
		}
	} else if trimmedShort := trimHexSuffix(short); trimmedShort != short {
		variants = append(variants, trimmedShort)
	}

	return uniqueSorted(variants)
}

func trimHexSuffix(value string) string {
	idx := strings.LastIndex(value, "-")
	if idx <= 0 {
		return value
	}
	suffix := value[idx+1:]
	if len(suffix) != 8 {
		return value
	}
	for _, r := range suffix {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return value
		}
	}
	return value[:idx]
}

func nodeMachineNames(node types.K8sNode) []string {
	keys := []string{
		node.MachineName,
		labelValue(node.Labels, "cluster.x-k8s.io/machine"),
		labelValue(node.Labels, "machine.cattle.io/name"),
		labelValue(node.Labels, "machine.cattle.io/machine"),
		labelValue(node.Labels, "cattle.io/machine"),
		labelValue(node.Labels, "cattle.io/machine-name"),
		labelValue(node.Labels, "rke.cattle.io/machine"),
		labelValue(node.Labels, "rke.cattle.io/machine-name"),
		labelValue(node.Labels, "management.cattle.io/machine"),
		labelValue(node.Labels, "provisioning.cattle.io/machine"),
		labelValue(node.Labels, "fleet.cattle.io/machine"),
		labelValue(node.Labels, "elemental.cattle.io/machine-name"),
		labelValue(node.Labels, "elemental.cattle.io/machine"),
		labelValue(node.Annotations, "cluster.x-k8s.io/machine"),
		labelValue(node.Annotations, "machine.cattle.io/name"),
		labelValue(node.Annotations, "machine.cattle.io/machine"),
		labelValue(node.Annotations, "cattle.io/machine"),
		labelValue(node.Annotations, "cattle.io/machine-name"),
		labelValue(node.Annotations, "rke.cattle.io/machine"),
		labelValue(node.Annotations, "rke.cattle.io/machine-name"),
		labelValue(node.Annotations, "management.cattle.io/machine"),
		labelValue(node.Annotations, "provisioning.cattle.io/machine"),
		labelValue(node.Annotations, "fleet.cattle.io/machine"),
		labelValue(node.Annotations, "elemental.cattle.io/machine-name"),
		labelValue(node.Annotations, "elemental.cattle.io/machine"),
	}

	var variants []string
	for _, key := range keys {
		if key == "" {
			continue
		}
		variants = append(variants, normalizedHostnames(normalizeMachineNameValue(key))...)
	}
	return uniqueSorted(variants)
}

func labelValue(values map[string]string, key string) string {
	if len(values) == 0 {
		return ""
	}
	value := strings.TrimSpace(values[key])
	if value == "" {
		return ""
	}
	return value
}

func normalizeMachineNameValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if idx := strings.LastIndex(value, "/"); idx >= 0 && idx < len(value)-1 {
		return strings.TrimSpace(value[idx+1:])
	}
	if idx := strings.LastIndex(value, ":"); idx >= 0 && idx < len(value)-1 {
		return strings.TrimSpace(value[idx+1:])
	}
	return value
}

func normalizedIPs(values []string) []string {
	var keys []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		ip := net.ParseIP(value)
		if ip == nil {
			continue
		}
		if v4 := ip.To4(); v4 != nil {
			keys = append(keys, v4.String())
			continue
		}
		keys = append(keys, ip.String())
	}
	return uniqueSorted(keys)
}

func uniqueSorted(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	set := make(map[string]struct{})
	for _, value := range values {
		set[value] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func nodeKey(node types.K8sNode) string {
	if node.UID != "" {
		return node.UID
	}
	return node.Name
}

func stableMatchSort(matches []NodeMatch) {
	sort.SliceStable(matches, func(i, j int) bool {
		left := matches[i].Node.Name
		right := matches[j].Node.Name
		return left < right
	})
}
