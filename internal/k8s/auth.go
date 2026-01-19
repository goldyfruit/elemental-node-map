package k8s

import (
	"fmt"
	"os"
	"strings"

	"k8s.io/client-go/tools/clientcmd"
)

// ExtractServerAndToken returns the cluster server URL and bearer token for the selected context.
func ExtractServerAndToken(clientConfig clientcmd.ClientConfig, contextName string) (string, string, error) {
	if clientConfig == nil {
		return "", "", fmt.Errorf("kubeconfig is required")
	}
	raw, err := clientConfig.RawConfig()
	if err != nil {
		return "", "", fmt.Errorf("failed to read kubeconfig: %w", err)
	}
	if contextName == "" {
		contextName = raw.CurrentContext
	}
	if contextName == "" {
		return "", "", fmt.Errorf("kubeconfig missing current context")
	}
	ctx, ok := raw.Contexts[contextName]
	if !ok {
		return "", "", fmt.Errorf("kubeconfig context not found: %s", contextName)
	}

	server := ""
	if cluster, ok := raw.Clusters[ctx.Cluster]; ok {
		server = strings.TrimSpace(cluster.Server)
	}

	token := ""
	var tokenErr error
	if auth, ok := raw.AuthInfos[ctx.AuthInfo]; ok {
		token = strings.TrimSpace(auth.Token)
		if token == "" && auth.TokenFile != "" {
			token, tokenErr = readTokenFile(auth.TokenFile)
		}
	}

	if server == "" || token == "" {
		restConfig, err := clientConfig.ClientConfig()
		if err == nil {
			if server == "" {
				server = strings.TrimSpace(restConfig.Host)
			}
			if token == "" {
				token = strings.TrimSpace(restConfig.BearerToken)
				if token == "" && restConfig.BearerTokenFile != "" {
					token, tokenErr = readTokenFile(restConfig.BearerTokenFile)
				}
			}
		} else if server == "" || token == "" {
			return "", "", fmt.Errorf("failed to resolve kubeconfig credentials: %w", err)
		}
	}

	if server == "" {
		return "", "", fmt.Errorf("kubeconfig missing cluster server for context %q", contextName)
	}
	if token == "" {
		if tokenErr != nil {
			return "", "", fmt.Errorf("failed to read kubeconfig token file: %w", tokenErr)
		}
		return "", "", fmt.Errorf("kubeconfig missing bearer token for context %q", contextName)
	}
	return server, token, nil
}

func readTokenFile(path string) (string, error) {
	content, err := os.ReadFile(expandPath(path))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(content)), nil
}
