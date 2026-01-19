package output

import (
	"sort"
	"strings"

	"github.com/goldyfruit/elemental-node-mapper/internal/k8s"
	"github.com/goldyfruit/elemental-node-mapper/internal/types"
	"github.com/pterm/pterm"
)

type NodesOptions struct {
	ShowLabels bool
	LabelKeys  []string
	Wide       bool
	Mode       Mode
}

func RenderNodes(nodes []types.K8sNode, opts NodesOptions) error {
	switch opts.Mode {
	case ModeJSON:
		return EmitJSON(nodes)
	case ModeYAML:
		return EmitYAML(nodes)
	default:
		return renderNodesTable(nodes, opts)
	}
}

func renderNodesTable(nodes []types.K8sNode, opts NodesOptions) error {
	InitStyles()
	columns := []string{"Node Name", "InternalIP"}
	if opts.Wide {
		columns = append(columns, "ExternalIP", "ProviderID", "MachineID")
	}
	if len(opts.LabelKeys) > 0 {
		for _, key := range opts.LabelKeys {
			columns = append(columns, key)
		}
	}
	if opts.ShowLabels {
		columns = append(columns, "Labels")
	}

	rows := make([][]string, 0, len(nodes))
	for _, node := range nodes {
		row := []string{node.Name, k8s.NodePrimaryInternalIP(node)}
		if opts.Wide {
			row = append(row, k8s.NodePrimaryExternalIP(node), node.ProviderID, node.MachineID)
		}
		if len(opts.LabelKeys) > 0 {
			for _, key := range opts.LabelKeys {
				row = append(row, valueOrDash(node.Labels[key]))
			}
		}
		if opts.ShowLabels {
			row = append(row, formatLabels(node.Labels))
		}
		rows = append(rows, row)
	}

	table := pterm.DefaultTable.WithHasHeader().WithData(append([][]string{columns}, rows...))
	return table.Render()
}

func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+labels[key])
	}
	return strings.Join(parts, ",")
}
