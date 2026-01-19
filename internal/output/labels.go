package output

import (
	"sort"

	"github.com/pterm/pterm"
)

type LabelKeySummary struct {
	Key   string `json:"key" yaml:"key"`
	Count int    `json:"count" yaml:"count"`
}

type LabelValueSummary struct {
	Value string `json:"value" yaml:"value"`
	Count int    `json:"count" yaml:"count"`
}

func RenderLabelKeys(counts map[string]int, mode Mode) error {
	list := make([]LabelKeySummary, 0, len(counts))
	for key, count := range counts {
		list = append(list, LabelKeySummary{Key: key, Count: count})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Key < list[j].Key })

	switch mode {
	case ModeJSON:
		return EmitJSON(list)
	case ModeYAML:
		return EmitYAML(list)
	default:
		return renderLabelKeyTable(list)
	}
}

func RenderLabelValues(key string, counts map[string]int, mode Mode) error {
	list := make([]LabelValueSummary, 0, len(counts))
	for value, count := range counts {
		list = append(list, LabelValueSummary{Value: value, Count: count})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Value < list[j].Value })

	switch mode {
	case ModeJSON:
		return EmitJSON(list)
	case ModeYAML:
		return EmitYAML(list)
	default:
		return renderLabelValueTable(key, list)
	}
}

func renderLabelKeyTable(list []LabelKeySummary) error {
	InitStyles()
	rows := [][]string{{"Label Key", "Count"}}
	for _, item := range list {
		rows = append(rows, []string{item.Key, formatCount(item.Count)})
	}
	table := pterm.DefaultTable.WithHasHeader().WithData(rows)
	return table.Render()
}

func renderLabelValueTable(key string, list []LabelValueSummary) error {
	InitStyles()
	rows := [][]string{{"Value", "Count"}}
	for _, item := range list {
		rows = append(rows, []string{item.Value, formatCount(item.Count)})
	}
	table := pterm.DefaultTable.WithHasHeader().WithData(rows)
	pterm.Println("Label values for", key)
	return table.Render()
}

func formatCount(count int) string {
	return pterm.DefaultBasicText.Sprint(count)
}
