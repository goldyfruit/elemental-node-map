package cmd

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/goldyfruit/elemental-node-mapper/internal/exit"
	"github.com/goldyfruit/elemental-node-mapper/internal/k8s"
	"github.com/goldyfruit/elemental-node-mapper/internal/match"
	"github.com/goldyfruit/elemental-node-mapper/internal/output"
	"github.com/goldyfruit/elemental-node-mapper/internal/rancher"
	"github.com/goldyfruit/elemental-node-mapper/internal/selector"
	"github.com/goldyfruit/elemental-node-mapper/internal/types"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
)

type hostResult struct {
	hosts []types.InventoryHost
	err   error
}

type machineResult struct {
	machines []rancher.Machine
	err      error
}

func newMatchCmd() *cobra.Command {
	var (
		rancherURL     string
		rancherToken   string
		rancherCluster string
		labelSearch    string
		selectorRaw    string
		showUnmatched  bool
		explain        bool
		wide           bool
		outputMode     string
		insecureTLS    bool
	)

	cmd := &cobra.Command{
		Use:   "match",
		Short: "Match inventory hosts with Kubernetes nodes",
		RunE: func(cmd *cobra.Command, args []string) error {
			mode, err := output.ParseMode(outputMode)
			if err != nil {
				return exit.New(1, err)
			}

			selectorParsed, err := selector.Parse(selectorRaw)
			if err != nil {
				return exit.New(1, err)
			}

			rancherURL = firstNonEmpty(rancherURL, os.Getenv("RANCHER_URL"))
			rancherToken = firstNonEmpty(rancherToken, os.Getenv("RANCHER_TOKEN"))
			rancherCluster = firstNonEmpty(rancherCluster, os.Getenv("RANCHER_CLUSTER"))

			if !cmd.Flags().Changed("insecure-skip-tls-verify") {
				if env := os.Getenv("RANCHER_INSECURE_SKIP_TLS_VERIFY"); env == "true" || env == "1" {
					insecureTLS = true
				}
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var (
				nodes       []types.K8sNode
				clusterName string
				kubeConfig  clientcmd.ClientConfig
				kubeInfo    k8s.KubeconfigInfo
				haveKube    bool
			)

			if rancherCluster == "" || rancherURL == "" || rancherToken == "" {
				var err error
				kubeConfig, kubeInfo, err = k8s.ResolveKubeconfig(kubeconfigPath, kubeContext)
				if err != nil {
					return exit.New(1, err)
				}
				haveKube = true
				if verbose {
					fmt.Fprintln(os.Stderr, k8s.DescribeKubeconfig(kubeInfo))
				}
			}

			if rancherURL == "" || rancherToken == "" {
				if !haveKube {
					return exit.New(1, fmt.Errorf("rancher URL or token missing and kubeconfig unavailable"))
				}
				server, token, err := k8s.ExtractServerAndToken(kubeConfig, kubeInfo.Context)
				if err != nil {
					return exit.New(1, err)
				}
				if rancherURL == "" {
					derived, err := rancher.InventoryURLFromServer(server)
					if err != nil {
						return exit.New(1, err)
					}
					rancherURL = derived
					if verbose {
						fmt.Fprintf(os.Stderr, "rancher url from kubeconfig=%s\n", rancherURL)
					}
				}
				if rancherToken == "" {
					rancherToken = token
					if verbose {
						fmt.Fprintln(os.Stderr, "rancher token from kubeconfig")
					}
				}
			}

			if rancherURL == "" {
				return exit.New(1, fmt.Errorf("rancher URL is required (use --rancher-url or RANCHER_URL)"))
			}
			if rancherToken == "" {
				return exit.New(1, fmt.Errorf("rancher token is required (use --rancher-token or RANCHER_TOKEN)"))
			}

			rancherClient, err := rancher.NewClient(rancherURL, rancherToken, insecureTLS)
			if err != nil {
				return exit.New(1, err)
			}

			hostsCh := make(chan hostResult, 1)
			go func() {
				hosts, err := rancherClient.ListInventoryHosts(ctx)
				hostsCh <- hostResult{hosts: hosts, err: err}
			}()

			var machinesCh chan machineResult
			if rancherCluster != "" {
				machinesURL, err := rancher.MachinesURLFromInventoryURL(rancherURL)
				if err != nil {
					return exit.New(1, err)
				}
				machinesClient, err := rancher.NewClient(machinesURL.String(), rancherToken, insecureTLS)
				if err != nil {
					return exit.New(1, err)
				}
				machinesCh = make(chan machineResult, 1)
				go func() {
					machines, err := machinesClient.ListMachines(ctx)
					machinesCh <- machineResult{machines: machines, err: err}
				}()
			}

			if rancherCluster != "" {
				managementURL, err := rancher.ClustersURLFromInventoryURL(rancherURL)
				if err != nil {
					return exit.New(1, err)
				}
				managementClient, err := rancher.NewClient(managementURL.String(), rancherToken, insecureTLS)
				if err != nil {
					return exit.New(1, err)
				}
				cluster, err := managementClient.ResolveCluster(ctx, rancherCluster)
				if err != nil {
					return exit.New(1, err)
				}
				clusterName = cluster.Name
				if clusterName == "" {
					clusterName = cluster.ID
				}
				cacheKey := rancher.KubeconfigCacheKey(managementURL.String(), cluster.ID)
				kubeconfigBytes, cacheAge, cacheHit, err := rancher.LoadCachedKubeconfig(cacheKey, rancher.DefaultKubeconfigCacheTTL)
				if err != nil && verbose {
					fmt.Fprintf(os.Stderr, "kubeconfig cache read failed: %v\n", err)
				}
				if cacheHit {
					if verbose {
						fmt.Fprintf(os.Stderr, "using cached kubeconfig age=%s\n", cacheAge.Truncate(time.Second))
					}
				} else {
					kubeconfigBytes, err = managementClient.GenerateKubeconfig(ctx, cluster.ID)
					if err != nil {
						return exit.New(2, err)
					}
					if err := rancher.SaveCachedKubeconfig(cacheKey, kubeconfigBytes); err != nil && verbose {
						fmt.Fprintf(os.Stderr, "kubeconfig cache write failed: %v\n", err)
					}
				}
				kubeConfig, info, err := k8s.ResolveKubeconfigFromBytes(kubeconfigBytes, "rancher", []string{"cluster:" + cluster.ID}, kubeContext)
				if err != nil {
					return exit.New(1, err)
				}
				if verbose {
					fmt.Fprintf(os.Stderr, "%s cluster=%s\n", k8s.DescribeKubeconfig(info), cluster.Name)
				}
				client, err := k8s.NewClient(kubeConfig)
				if err != nil {
					return exit.New(1, err)
				}
				nodes, err = client.ListNodes(ctx, selectorParsed)
				if err != nil {
					return exit.New(2, err)
				}
				if machinesCh != nil {
					result := <-machinesCh
					if result.err != nil {
						if verbose {
							fmt.Fprintf(os.Stderr, "rancher machine lookup skipped: %v\n", result.err)
						}
					} else {
						nameByNode := rancher.MachineNameMap(result.machines, cluster.Name)
						if len(nameByNode) > 0 {
							for i := range nodes {
								if name := nameByNode[nodes[i].Name]; name != "" {
									nodes[i].MachineName = name
								}
							}
						}
					}
				}
			} else {
				if !haveKube {
					return exit.New(1, fmt.Errorf("kubeconfig is required to list nodes"))
				}
				client, err := k8s.NewClient(kubeConfig)
				if err != nil {
					return exit.New(1, err)
				}
				nodes, err = client.ListNodes(ctx, selectorParsed)
				if err != nil {
					return exit.New(2, err)
				}
			}
			if labelSearch != "" {
				patterns := parseLabelKeys(labelSearch)
				filtered, err := filterNodesByLabelPatterns(nodes, patterns)
				if err != nil {
					return exit.New(1, err)
				}
				if verbose {
					fmt.Fprintf(os.Stderr, "label filter matched %d/%d nodes\n", len(filtered), len(nodes))
				}
				nodes = filtered
			}
			hostResult := <-hostsCh
			if hostResult.err != nil {
				return exit.New(2, hostResult.err)
			}
			hosts := hostResult.hosts

			result := match.Match(hosts, nodes)
			opts := output.MatchOptions{
				ShowUnmatched: showUnmatched,
				Explain:       explain,
				Wide:          wide,
				Mode:          mode,
				ClusterName:   clusterName,
			}
			if err := output.RenderMatch(result, opts); err != nil {
				return exit.New(1, err)
			}
			if len(result.Ambiguous) > 0 {
				return exit.New(3, fmt.Errorf("ambiguous matches present"))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&rancherURL, "rancher-url", "", "Rancher inventory API URL")
	cmd.Flags().StringVar(&rancherToken, "rancher-token", "", "Rancher API bearer token")
	cmd.Flags().StringVar(&rancherCluster, "rancher-cluster", "", "downstream cluster name or ID (resolved via Rancher)")
	cmd.Flags().StringVar(&labelSearch, "labels", "", "filter nodes by label key/value (comma-separated, supports * or /regex/)")
	cmd.Flags().StringVar(&selectorRaw, "selector", "", "label selector to filter nodes")
	cmd.Flags().BoolVar(&showUnmatched, "show-unmatched", false, "show unmatched hosts and nodes")
	cmd.Flags().BoolVar(&explain, "explain", false, "include match explanations")
	cmd.Flags().BoolVar(&wide, "wide", false, "show wide output")
	cmd.Flags().StringVar(&outputMode, "output", "table", "output format: table|json|yaml")
	cmd.Flags().BoolVar(&insecureTLS, "insecure-skip-tls-verify", false, "skip TLS verification for Rancher")

	return cmd
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

type labelMatcher func(string) bool

func filterNodesByLabelPatterns(nodes []types.K8sNode, patterns []string) ([]types.K8sNode, error) {
	matchers, err := buildLabelMatchers(patterns)
	if err != nil || len(matchers) == 0 {
		return nodes, err
	}
	filtered := make([]types.K8sNode, 0, len(nodes))
	for _, node := range nodes {
		if labelsMatchAny(node.Labels, matchers) {
			filtered = append(filtered, node)
		}
	}
	return filtered, nil
}

func buildLabelMatchers(patterns []string) ([]labelMatcher, error) {
	var matchers []labelMatcher
	for _, raw := range patterns {
		pattern := strings.TrimSpace(raw)
		if pattern == "" {
			continue
		}
		if expr, ok := regexPattern(pattern); ok {
			rx, err := regexp.Compile("(?i)" + expr)
			if err != nil {
				return nil, fmt.Errorf("invalid label regex %q: %w", pattern, err)
			}
			matchers = append(matchers, rx.MatchString)
			continue
		}
		if strings.ContainsAny(pattern, "*?") {
			expr := "(?i)" + wildcardToRegex(pattern)
			rx, err := regexp.Compile(expr)
			if err != nil {
				return nil, fmt.Errorf("invalid label pattern %q: %w", pattern, err)
			}
			matchers = append(matchers, rx.MatchString)
			continue
		}
		pattern = strings.ToLower(pattern)
		matchers = append(matchers, func(value string) bool {
			return strings.Contains(strings.ToLower(value), pattern)
		})
	}
	return matchers, nil
}

func labelsMatchAny(labels map[string]string, matchers []labelMatcher) bool {
	if len(labels) == 0 || len(matchers) == 0 {
		return false
	}
	for key, value := range labels {
		for _, match := range matchers {
			if match(key) || match(value) {
				return true
			}
		}
	}
	return false
}
