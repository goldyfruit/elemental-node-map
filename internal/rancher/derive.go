package rancher

import (
	"fmt"
	"net/url"
	"strings"
)

const DefaultInventoryPath = "/v1/elemental.cattle.io.machineinventories"

func InventoryURLFromServer(server string) (string, error) {
	base, err := BaseURLFromServer(server)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(base, "/") + DefaultInventoryPath, nil
}

func BaseURLFromServer(server string) (string, error) {
	parsed, err := url.Parse(server)
	if err != nil {
		return "", fmt.Errorf("invalid kubeconfig server URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid kubeconfig server URL: %s", server)
	}
	path := parsed.Path
	if idx := strings.Index(path, "/k8s/"); idx >= 0 {
		path = path[:idx]
	} else if idx := strings.Index(path, "/k8s"); idx >= 0 {
		path = path[:idx]
	}
	parsed.Path = strings.TrimSuffix(path, "/")
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}
