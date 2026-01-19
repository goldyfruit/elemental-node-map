package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/goldyfruit/elemental-node-mapper/internal/exit"
	"github.com/goldyfruit/elemental-node-mapper/internal/k8s"
	"github.com/goldyfruit/elemental-node-mapper/internal/output"
	"github.com/goldyfruit/elemental-node-mapper/internal/types"
	"github.com/spf13/cobra"
)

func newLabelsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "labels",
		Short: "Explore node label usage",
	}

	cmd.AddCommand(newLabelsKeysCmd())
	cmd.AddCommand(newLabelsValuesCmd())

	return cmd
}

func newLabelsKeysCmd() *cobra.Command {
	var outputMode string
	cmd := &cobra.Command{
		Use:   "keys",
		Short: "List node label keys",
		RunE: func(cmd *cobra.Command, args []string) error {
			mode, err := output.ParseMode(outputMode)
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
			nodes, err := client.ListNodes(ctx, nil)
			if err != nil {
				return exit.New(2, err)
			}

			counts := countLabelKeys(nodes)
			if err := output.RenderLabelKeys(counts, mode); err != nil {
				return exit.New(1, err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&outputMode, "output", "table", "output format: table|json|yaml")
	return cmd
}

func newLabelsValuesCmd() *cobra.Command {
	var outputMode string
	cmd := &cobra.Command{
		Use:   "values <key>",
		Short: "List values for a label key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode, err := output.ParseMode(outputMode)
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
			nodes, err := client.ListNodes(ctx, nil)
			if err != nil {
				return exit.New(2, err)
			}

			key := args[0]
			counts := countLabelValues(nodes, key)
			if err := output.RenderLabelValues(key, counts, mode); err != nil {
				return exit.New(1, err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&outputMode, "output", "table", "output format: table|json|yaml")
	return cmd
}

func countLabelKeys(nodes []types.K8sNode) map[string]int {
	counts := map[string]int{}
	for _, node := range nodes {
		for key := range node.Labels {
			counts[key]++
		}
	}
	return counts
}

func countLabelValues(nodes []types.K8sNode, key string) map[string]int {
	counts := map[string]int{}
	for _, node := range nodes {
		if value, ok := node.Labels[key]; ok {
			counts[value]++
		}
	}
	return counts
}
