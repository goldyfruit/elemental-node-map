package k8s

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/goldyfruit/elemental-node-mapper/internal/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	clientset *kubernetes.Clientset
}

func NewClient(clientConfig clientcmd.ClientConfig) (*Client, error) {
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, &ConfigError{Kind: ErrKubeconfigInvalid, Err: err}
	}
	config.Timeout = 15 * time.Second
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, &APIError{Kind: ErrUnknown, Err: err}
	}
	return &Client{clientset: clientset}, nil
}

func (c *Client) ListNodes(ctx context.Context, selector labels.Selector) ([]types.K8sNode, error) {
	if selector == nil {
		selector = labels.Everything()
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	nodes, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, classifyK8sError(err)
	}
	return normalizeNodes(nodes.Items), nil
}

func normalizeNodes(items []v1.Node) []types.K8sNode {
	out := make([]types.K8sNode, 0, len(items))
	for _, node := range items {
		out = append(out, normalizeNode(node))
	}
	return out
}

func normalizeNode(node v1.Node) types.K8sNode {
	internalIPs := []string{}
	externalIPs := []string{}
	for _, addr := range node.Status.Addresses {
		switch addr.Type {
		case v1.NodeInternalIP:
			internalIPs = append(internalIPs, addr.Address)
		case v1.NodeExternalIP:
			externalIPs = append(externalIPs, addr.Address)
		}
	}

	labels := map[string]string{}
	for key, value := range node.Labels {
		labels[key] = value
	}
	annotations := map[string]string{}
	for key, value := range node.Annotations {
		annotations[key] = value
	}

	return types.K8sNode{
		Name:        node.Name,
		UID:         string(node.UID),
		Labels:      labels,
		ProviderID:  node.Spec.ProviderID,
		MachineID:   node.Status.NodeInfo.MachineID,
		MachineName: nodeMachineName(labels, annotations),
		InternalIPs: internalIPs,
		ExternalIPs: externalIPs,
		Annotations: annotations,
	}
}

func NodePrimaryInternalIP(node types.K8sNode) string {
	if len(node.InternalIPs) == 0 {
		return ""
	}
	return node.InternalIPs[0]
}

func NodePrimaryExternalIP(node types.K8sNode) string {
	if len(node.ExternalIPs) == 0 {
		return ""
	}
	return node.ExternalIPs[0]
}

func nodeMachineName(labels map[string]string, annotations map[string]string) string {
	candidates := machineNameCandidates(labels, annotations)
	for _, candidate := range candidates {
		if candidate != "" && !isUUID(candidate) {
			return candidate
		}
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return ""
}

func machineNameCandidates(labels map[string]string, annotations map[string]string) []string {
	keys := []string{
		"cluster.x-k8s.io/machine",
		"machine.cattle.io/name",
		"machine.cattle.io/machine",
		"cattle.io/machine",
		"cattle.io/machine-name",
		"rke.cattle.io/machine",
		"rke.cattle.io/machine-name",
		"management.cattle.io/machine",
		"provisioning.cattle.io/machine",
		"fleet.cattle.io/machine",
		"elemental.cattle.io/machine-name",
		"elemental.cattle.io/machine",
	}

	var candidates []string
	for _, key := range keys {
		if value := strings.TrimSpace(labels[key]); value != "" {
			candidates = append(candidates, normalizeMachineName(value))
		}
	}
	for _, key := range keys {
		if value := strings.TrimSpace(annotations[key]); value != "" {
			candidates = append(candidates, normalizeMachineName(value))
		}
	}
	return uniqueStrings(candidates)
}

func normalizeMachineName(value string) string {
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

func isUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, r := range value {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if !isHexRune(r) {
				return false
			}
		}
	}
	return true
}

func isHexRune(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func (c *Client) ServerVersion(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	version, err := c.clientset.Discovery().ServerVersion()
	if err != nil {
		return "", classifyK8sError(err)
	}
	return fmt.Sprintf("%s.%s", version.Major, version.Minor), nil
}
