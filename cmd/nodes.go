package cmd

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/goldyfruit/elemental-node-mapper/internal/exit"
	"github.com/goldyfruit/elemental-node-mapper/internal/k8s"
	"github.com/goldyfruit/elemental-node-mapper/internal/output"
	"github.com/goldyfruit/elemental-node-mapper/internal/selector"
	"github.com/goldyfruit/elemental-node-mapper/internal/types"
	"github.com/spf13/cobra"
)

func newNodesCmd() *cobra.Command {
	var (
		selectorRaw string
		showLabels  bool
		labelKeys   string
		wide        bool
		outputMode  string
	)

	cmd := &cobra.Command{
		Use:   "nodes",
		Short: "List Kubernetes nodes",
		RunE: func(cmd *cobra.Command, args []string) error {
			mode, err := output.ParseMode(outputMode)
			if err != nil {
				return exit.New(1, err)
			}

			selectorParsed, err := selector.Parse(selectorRaw)
			if err != nil {
				return exit.New(1, err)
			}

			kubeConfig, info, err := k8s.ResolveKubeconfig(kubeconfigPath, kubeContext)
			if err != nil {
				return exit.New(1, err)
			}
			if verbose {
				fmt.Fprintln(os.Stderr, k8s.DescribeKubeconfig(info))
			}

			client, err := k8s.NewClient(kubeConfig)
			if err != nil {
				return exit.New(1, err)
			}

			ctx := context.Background()
			nodes, err := client.ListNodes(ctx, selectorParsed)
			if err != nil {
				return exit.New(2, err)
			}

			keys := parseLabelKeys(labelKeys)
			if len(keys) > 0 {
				expanded, err := expandLabelKeys(nodes, keys)
				if err != nil {
					return exit.New(1, err)
				}
				keys = expanded
			}
			opts := output.NodesOptions{
				ShowLabels: showLabels,
				LabelKeys:  keys,
				Wide:       wide,
				Mode:       mode,
			}
			if err := output.RenderNodes(nodes, opts); err != nil {
				return exit.New(1, err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&selectorRaw, "selector", "", "label selector to filter nodes")
	cmd.Flags().BoolVar(&showLabels, "labels", false, "show all labels in output")
	cmd.Flags().StringVar(&labelKeys, "label-keys", "", "comma-separated label keys or patterns (exact key, * wildcard, or /regex/)")
	cmd.Flags().BoolVar(&wide, "wide", false, "show wide output")
	cmd.Flags().StringVar(&outputMode, "output", "table", "output format: table|json|yaml")

	return cmd
}

func parseLabelKeys(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	keys := make([]string, 0, len(parts))
	for _, part := range parts {
		key := strings.TrimSpace(part)
		if key != "" {
			keys = append(keys, key)
		}
	}
	return keys
}

func expandLabelKeys(nodes []types.K8sNode, patterns []string) ([]string, error) {
	allKeys := collectLabelKeys(nodes)
	if len(allKeys) == 0 {
		return patterns, nil
	}
	out := []string{}
	seen := map[string]struct{}{}

	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if expr, ok := regexPattern(pattern); ok {
			rx, err := regexp.Compile(expr)
			if err != nil {
				return nil, fmt.Errorf("invalid label regex %q: %w", pattern, err)
			}
			appendRegexMatches(&out, seen, allKeys, rx)
			continue
		}
		if strings.ContainsAny(pattern, "*?") {
			expr := wildcardToRegex(pattern)
			rx, err := regexp.Compile(expr)
			if err != nil {
				return nil, fmt.Errorf("invalid label pattern %q: %w", pattern, err)
			}
			appendRegexMatches(&out, seen, allKeys, rx)
			continue
		}
		appendIfMissing(&out, seen, pattern)
	}

	return out, nil
}

func collectLabelKeys(nodes []types.K8sNode) []string {
	set := map[string]struct{}{}
	for _, node := range nodes {
		for key := range node.Labels {
			set[key] = struct{}{}
		}
	}
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func regexPattern(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "re:") {
		return strings.TrimSpace(raw[3:]), true
	}
	if strings.HasPrefix(raw, "regex:") {
		return strings.TrimSpace(raw[6:]), true
	}
	if len(raw) >= 2 && strings.HasPrefix(raw, "/") && strings.HasSuffix(raw, "/") {
		return raw[1 : len(raw)-1], true
	}
	return "", false
}

func wildcardToRegex(pattern string) string {
	quoted := regexp.QuoteMeta(pattern)
	quoted = strings.ReplaceAll(quoted, "\\*", ".*")
	quoted = strings.ReplaceAll(quoted, "\\?", ".")
	return "^" + quoted + "$"
}

func appendRegexMatches(out *[]string, seen map[string]struct{}, keys []string, rx *regexp.Regexp) {
	for _, key := range keys {
		if rx.MatchString(key) {
			appendIfMissing(out, seen, key)
		}
	}
}

func appendIfMissing(out *[]string, seen map[string]struct{}, value string) {
	if _, ok := seen[value]; ok {
		return
	}
	seen[value] = struct{}{}
	*out = append(*out, value)
}
