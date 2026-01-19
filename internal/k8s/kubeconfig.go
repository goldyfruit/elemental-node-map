package k8s

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/client-go/tools/clientcmd"
)

type KubeconfigInfo struct {
	Source  string
	Paths   []string
	Context string
}

func ResolveKubeconfig(explicitPath, contextOverride string) (clientcmd.ClientConfig, KubeconfigInfo, error) {
	info := KubeconfigInfo{}
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	paths := []string{}

	switch {
	case explicitPath != "":
		info.Source = "flag"
		paths = []string{expandPath(explicitPath)}
		rules.ExplicitPath = paths[0]
	case os.Getenv("KUBECONFIG") != "":
		info.Source = "env"
		paths = expandPaths(filepath.SplitList(os.Getenv("KUBECONFIG")))
		rules.Precedence = paths
	case len(rules.Precedence) > 0:
		info.Source = "default"
		paths = expandPaths(rules.Precedence)
	default:
		info.Source = "default"
	}
	info.Paths = paths

	existing := existingPaths(paths)
	if len(existing) == 0 {
		return nil, info, &ConfigError{Kind: ErrKubeconfigNotFound, Paths: paths}
	}

	overrides := &clientcmd.ConfigOverrides{}
	if contextOverride != "" {
		overrides.CurrentContext = contextOverride
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)
	rawConfig, err := clientConfig.RawConfig()
	if err != nil {
		return nil, info, &ConfigError{Kind: ErrKubeconfigInvalid, Paths: paths, Err: err}
	}

	contextName := overrides.CurrentContext
	if contextName == "" {
		contextName = rawConfig.CurrentContext
	}
	if contextName == "" {
		return nil, info, &ConfigError{Kind: ErrKubeconfigInvalid, Paths: paths, Err: fmt.Errorf("missing current context")}
	}
	info.Context = contextName
	if _, ok := rawConfig.Contexts[contextName]; !ok {
		return nil, info, &ConfigError{Kind: ErrContextNotFound, Paths: paths, Err: fmt.Errorf("context %q not found", contextName)}
	}
	return clientConfig, info, nil
}

func ResolveKubeconfigFromBytes(content []byte, source string, paths []string, contextOverride string) (clientcmd.ClientConfig, KubeconfigInfo, error) {
	info := KubeconfigInfo{Source: source, Paths: paths}
	if info.Source == "" {
		info.Source = "inline"
	}
	rawConfig, err := clientcmd.Load(content)
	if err != nil {
		return nil, info, &ConfigError{Kind: ErrKubeconfigInvalid, Paths: paths, Err: err}
	}

	overrides := &clientcmd.ConfigOverrides{}
	if contextOverride != "" {
		overrides.CurrentContext = contextOverride
	}

	contextName := overrides.CurrentContext
	if contextName == "" {
		contextName = rawConfig.CurrentContext
	}
	if contextName == "" {
		return nil, info, &ConfigError{Kind: ErrKubeconfigInvalid, Paths: paths, Err: fmt.Errorf("missing current context")}
	}
	info.Context = contextName
	if _, ok := rawConfig.Contexts[contextName]; !ok {
		return nil, info, &ConfigError{Kind: ErrContextNotFound, Paths: paths, Err: fmt.Errorf("context %q not found", contextName)}
	}

	clientConfig := clientcmd.NewNonInteractiveClientConfig(*rawConfig, contextName, overrides, nil)
	return clientConfig, info, nil
}

func existingPaths(paths []string) []string {
	var existing []string
	for _, path := range paths {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			existing = append(existing, path)
		}
	}
	return existing
}

func expandPaths(paths []string) []string {
	if len(paths) == 0 {
		return nil
	}
	expanded := make([]string, 0, len(paths))
	for _, path := range paths {
		expanded = append(expanded, expandPath(path))
	}
	return expanded
}

func expandPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return path
	}
	if path[0] != '~' {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}

func DescribeKubeconfig(info KubeconfigInfo) string {
	paths := strings.Join(info.Paths, string(os.PathListSeparator))
	if paths == "" {
		paths = "(none)"
	}
	return fmt.Sprintf("kubeconfig source=%s paths=%s context=%s", info.Source, paths, info.Context)
}
