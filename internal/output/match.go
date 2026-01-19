package output

import (
	"fmt"
	"strings"

	"github.com/goldyfruit/elemental-node-mapper/internal/k8s"
	"github.com/goldyfruit/elemental-node-mapper/internal/match"
	"github.com/goldyfruit/elemental-node-mapper/internal/types"
	"github.com/pterm/pterm"
)

type MatchOptions struct {
	ShowUnmatched bool
	Explain       bool
	Wide          bool
	Mode          Mode
	ClusterName   string
}

type MatchSummary struct {
	Matched        int `json:"matched" yaml:"matched"`
	Ambiguous      int `json:"ambiguous" yaml:"ambiguous"`
	UnmatchedHosts int `json:"unmatchedHosts" yaml:"unmatchedHosts"`
	UnmatchedNodes int `json:"unmatchedNodes" yaml:"unmatchedNodes"`
}

type MatchOutput struct {
	Cluster        string                `json:"cluster,omitempty" yaml:"cluster,omitempty"`
	Summary        MatchSummary          `json:"summary" yaml:"summary"`
	Matches        []matchPayload        `json:"matches" yaml:"matches"`
	Ambiguous      []matchPayload        `json:"ambiguous" yaml:"ambiguous"`
	UnmatchedHosts []types.InventoryHost `json:"unmatchedHosts" yaml:"unmatchedHosts"`
	UnmatchedNodes []types.K8sNode       `json:"unmatchedNodes" yaml:"unmatchedNodes"`
}

type matchPayload struct {
	Host       types.InventoryHost `json:"host" yaml:"host"`
	Candidates []matchCandidate    `json:"candidates" yaml:"candidates"`
	Method     match.Method        `json:"method" yaml:"method"`
	Confidence float64             `json:"confidence" yaml:"confidence"`
}

type matchCandidate struct {
	Node        types.K8sNode `json:"node" yaml:"node"`
	Method      match.Method  `json:"method" yaml:"method"`
	Confidence  float64       `json:"confidence" yaml:"confidence"`
	Explanation string        `json:"explanation,omitempty" yaml:"explanation,omitempty"`
}

func RenderMatch(result match.Result, opts MatchOptions) error {
	summary := MatchSummary{
		Matched:        len(result.Matches),
		Ambiguous:      len(result.Ambiguous),
		UnmatchedHosts: len(result.UnmatchedHosts),
		UnmatchedNodes: len(result.UnmatchedNodes),
	}

	switch opts.Mode {
	case ModeJSON:
		return EmitJSON(buildMatchOutput(result, summary, opts.Explain, opts.ClusterName))
	case ModeYAML:
		return EmitYAML(buildMatchOutput(result, summary, opts.Explain, opts.ClusterName))
	default:
		return renderMatchTable(result, summary, opts)
	}
}

func buildMatchOutput(result match.Result, summary MatchSummary, explain bool, clusterName string) MatchOutput {
	payload := MatchOutput{
		Cluster:        clusterName,
		Summary:        summary,
		UnmatchedHosts: result.UnmatchedHosts,
		UnmatchedNodes: result.UnmatchedNodes,
	}
	payload.Matches = renderMatchPayload(result.Matches, explain)
	payload.Ambiguous = renderMatchPayload(result.Ambiguous, explain)
	return payload
}

func renderMatchPayload(matches []match.HostMatch, explain bool) []matchPayload {
	out := make([]matchPayload, 0, len(matches))
	for _, entry := range matches {
		payload := matchPayload{
			Host:       entry.Host,
			Method:     entry.Method,
			Confidence: entry.Confidence,
		}
		for _, candidate := range entry.Candidates {
			outCandidate := matchCandidate{
				Node:       candidate.Node,
				Method:     candidate.Method,
				Confidence: candidate.Confidence,
			}
			if explain {
				outCandidate.Explanation = candidate.Explanation
			}
			payload.Candidates = append(payload.Candidates, outCandidate)
		}
		out = append(out, payload)
	}
	return out
}

func renderMatchTable(result match.Result, summary MatchSummary, opts MatchOptions) error {
	InitStyles()
	renderSummaryBox(summary, opts.ClusterName)
	renderLegend()

	hasMatches := len(result.Matches)+len(result.Ambiguous) > 0
	if hasMatches {
		if err := renderMatchesTable(result, opts); err != nil {
			return err
		}
	} else {
		pterm.Println("No matches found between Elemental inventory and Kubernetes nodes.")
	}

	if !opts.ShowUnmatched {
		return nil
	}

	if len(result.UnmatchedHosts) > 0 {
		if err := renderUnmatchedHostsTable(result.UnmatchedHosts, opts); err != nil {
			return err
		}
	}
	if len(result.UnmatchedNodes) > 0 {
		if err := renderUnmatchedNodesTable(result.UnmatchedNodes, opts); err != nil {
			return err
		}
	}
	return nil
}

func hostLabel(host types.InventoryHost) string {
	if host.MachineName != "" {
		return host.MachineName
	}
	if host.Hostname != "" {
		return host.Hostname
	}
	if host.ID != "" {
		return host.ID
	}
	if host.UID != "" {
		return host.UID
	}
	return "(unknown)"
}

func renderMatchesTable(result match.Result, opts MatchOptions) error {
	sectionTitle("Matches")
	rows := [][]string{}
	columns := []string{"Status", "Elemental Host", "Rancher Machine", "K8s Node", "Match Method", "Confidence", "K8s InternalIP"}
	if opts.Wide {
		columns = append(columns, "K8s ExternalIP", "K8s ProviderID", "K8s MachineID")
	}
	if opts.Explain {
		columns = append(columns, "Why")
	}

	appendRow := func(status string, host string, node types.K8sNode, method match.Method, confidence float64, explanation string) {
		rancherMachine := node.MachineName
		if rancherMachine == "" {
			rancherMachine = host
		}
		row := []string{
			status,
			host,
			valueOrDash(rancherMachine),
			node.Name,
			string(method),
			fmt.Sprintf("%.0f%%", confidence*100),
			k8s.NodePrimaryInternalIP(node),
		}
		if opts.Wide {
			row = append(row, valueOrDash(k8s.NodePrimaryExternalIP(node)), valueOrDash(node.ProviderID), valueOrDash(node.MachineID))
		}
		if opts.Explain {
			row = append(row, valueOrDash(explanation))
		}
		rows = append(rows, row)
	}

	for _, entry := range result.Matches {
		for _, candidate := range entry.Candidates {
			appendRow(statusBadge("MATCHED", pterm.BgGreen, pterm.FgBlack), hostLabel(entry.Host), candidate.Node, candidate.Method, candidate.Confidence, candidate.Explanation)
		}
	}
	for _, entry := range result.Ambiguous {
		for _, candidate := range entry.Candidates {
			appendRow(statusBadge("AMBIG", pterm.BgYellow, pterm.FgBlack), hostLabel(entry.Host), candidate.Node, candidate.Method, candidate.Confidence, candidate.Explanation)
		}
	}

	table := styledTable(append([][]string{columns}, rows...))
	return table.Render()
}

func renderUnmatchedHostsTable(hosts []types.InventoryHost, opts MatchOptions) error {
	sectionTitle("Unmatched Hosts")
	columns := []string{"Elemental Host", "Inventory ID", "Hostname", "IPs"}
	if opts.Wide {
		columns = append(columns, "Namespace", "Inventory UID", "Host MachineID", "Host SystemUUID", "Host ProviderID")
	}
	if opts.Explain {
		columns = append(columns, "Why")
	}

	rows := make([][]string, 0, len(hosts))
	for i, host := range hosts {
		label := hostLabel(host)
		if label == "(unknown)" {
			label = fmt.Sprintf("(empty record #%d)", i+1)
		}
		row := []string{
			label,
			valueOrDash(host.ID),
			valueOrDash(host.Hostname),
			valueOrDash(joinValues(host.IPs)),
		}
		if opts.Wide {
			row = append(row, valueOrDash(host.Namespace), valueOrDash(host.UID), valueOrDash(host.MachineID), valueOrDash(host.SystemUUID), valueOrDash(host.ProviderID))
		}
		if opts.Explain {
			reason := "no Kubernetes node match"
			if host.MachineName == "" && host.Hostname == "" && host.ID == "" && host.UID == "" &&
				len(host.IPs) == 0 && host.MachineID == "" && host.SystemUUID == "" && host.ProviderID == "" {
				reason = "inventory record missing identifiers"
			}
			row = append(row, reason)
		}
		rows = append(rows, row)
	}

	table := styledTable(append([][]string{columns}, rows...))
	return table.Render()
}

func renderUnmatchedNodesTable(nodes []types.K8sNode, opts MatchOptions) error {
	sectionTitle("Unmatched Nodes")
	columns := []string{"Rancher Machine", "K8s Node", "K8s InternalIP"}
	if opts.Wide {
		columns = append(columns, "K8s ExternalIP", "K8s ProviderID", "K8s MachineID")
	}
	if opts.Explain {
		columns = append(columns, "Why")
	}

	rows := make([][]string, 0, len(nodes))
	for _, node := range nodes {
		row := []string{
			valueOrDash(node.MachineName),
			node.Name,
			valueOrDash(k8s.NodePrimaryInternalIP(node)),
		}
		if opts.Wide {
			row = append(row, valueOrDash(k8s.NodePrimaryExternalIP(node)), valueOrDash(node.ProviderID), valueOrDash(node.MachineID))
		}
		if opts.Explain {
			row = append(row, "no Elemental inventory match")
		}
		rows = append(rows, row)
	}

	table := styledTable(append([][]string{columns}, rows...))
	return table.Render()
}

func joinValues(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return strings.Join(values, ",")
}

func valueOrDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func renderSummaryBox(summary MatchSummary, clusterName string) {
	lines := []string{}
	if clusterName != "" {
		lines = append(lines, pterm.FgGray.Sprint("Cluster: ")+pterm.FgLightCyan.Sprint(clusterName))
	}

	stats := []string{
		metricBadge("Matched", summary.Matched, pterm.FgLightGreen),
		metricBadge("Ambiguous", summary.Ambiguous, pterm.FgLightYellow),
		metricBadge("Unmatched Hosts", summary.UnmatchedHosts, pterm.FgLightRed),
		metricBadge("Unmatched Nodes", summary.UnmatchedNodes, pterm.FgLightRed),
	}
	lines = append(lines, strings.Join(stats, "  "))

	box := pterm.DefaultBox.WithTitle("elemental-node-map match").WithTitleTopCenter(true)
	box.BoxStyle = pterm.NewStyle(pterm.FgLightCyan)
	box.Println(strings.Join(lines, "\n"))
}

func metricBadge(label string, value int, color pterm.Color) string {
	style := pterm.NewStyle(color, pterm.Bold)
	return style.Sprintf("%s: %d", label, value)
}

func sectionTitle(title string) {
	style := pterm.NewStyle(pterm.Bold, pterm.FgLightCyan)
	pterm.Println(style.Sprint(title))
}

func styledTable(data [][]string) *pterm.TablePrinter {
	headerStyle := pterm.NewStyle(pterm.Bold, pterm.FgLightCyan)
	sepStyle := pterm.NewStyle(pterm.FgDarkGray)
	altStyle := pterm.NewStyle(pterm.FgGray)

	table := pterm.DefaultTable.WithHasHeader().WithData(data).WithBoxed(true)
	table.HeaderStyle = headerStyle
	table.SeparatorStyle = sepStyle
	table.HeaderRowSeparator = "-"
	table.HeaderRowSeparatorStyle = sepStyle
	table.AlternateRowStyle = altStyle
	return table
}

func statusBadge(label string, bg pterm.Color, fg pterm.Color) string {
	style := pterm.NewStyle(bg, fg, pterm.Bold)
	return style.Sprint(" " + label + " ")
}

func renderLegend() {
	style := pterm.NewStyle(pterm.FgGray)
	pterm.Println(style.Sprint("Legend: Elemental Host = inventory record from Rancher Elemental."))
	pterm.Println(style.Sprint("        Rancher Machine = machine resource name; K8s Node = downstream Kubernetes node."))
}
