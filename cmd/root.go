package cmd

import (
	"github.com/spf13/cobra"
)

var (
	kubeconfigPath string
	kubeContext    string
	verbose        bool
)

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "elemental-node-map",
		Short:         "Match Elemental inventory hosts with Kubernetes nodes",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.PersistentFlags().StringVar(&kubeconfigPath, "kubeconfig", "", "path to kubeconfig file")
	cmd.PersistentFlags().StringVar(&kubeContext, "context", "", "kubeconfig context to use")
	cmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose logging")

	cmd.AddCommand(newMatchCmd())
	cmd.AddCommand(newNodesCmd())
	cmd.AddCommand(newLabelsCmd())

	return cmd
}
