package rancher

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"
)

type Cluster struct {
	ID   string
	Name string
}

func ClustersURLFromInventoryURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid rancher URL: %w", err)
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	basePath := strings.TrimSuffix(stripAPISuffix(parsed.Path), "/")
	if basePath == "" {
		parsed.Path = "/v3/clusters"
	} else {
		parsed.Path = basePath + "/v3/clusters"
	}
	return parsed, nil
}

func (c *Client) ListClusters(ctx context.Context) ([]Cluster, error) {
	var clusters []Cluster
	nextURL := c.withLimit(c.baseURL, 200)

	for nextURL != nil {
		page, err := c.fetchPage(ctx, nextURL)
		if err != nil {
			return nil, err
		}
		for _, raw := range page.Data {
			clusters = append(clusters, normalizeCluster(raw))
		}
		nextURL = page.NextURL(c.baseURL)
	}

	return clusters, nil
}

func (c *Client) ResolveCluster(ctx context.Context, identifier string) (Cluster, error) {
	if identifier == "" {
		return Cluster{}, fmt.Errorf("rancher cluster is required")
	}
	clusters, err := c.ListClusters(ctx)
	if err != nil {
		return Cluster{}, err
	}

	var nameMatches []Cluster
	for _, cluster := range clusters {
		if cluster.ID == identifier {
			return cluster, nil
		}
		if cluster.Name == identifier {
			nameMatches = append(nameMatches, cluster)
		}
	}

	switch len(nameMatches) {
	case 1:
		return nameMatches[0], nil
	case 0:
		return Cluster{}, fmt.Errorf("cluster %q not found", identifier)
	default:
		return Cluster{}, fmt.Errorf("multiple clusters named %q: %s", identifier, joinClusterIDs(nameMatches))
	}
}

func (c *Client) GenerateKubeconfig(ctx context.Context, clusterID string) ([]byte, error) {
	if clusterID == "" {
		return nil, fmt.Errorf("cluster ID is required")
	}
	if c.baseURL == nil {
		return nil, fmt.Errorf("rancher base URL is required")
	}

	target := *c.baseURL
	target.Path = strings.TrimSuffix(target.Path, "/") + "/" + clusterID
	q := target.Query()
	q.Set("action", "generateKubeconfig")
	target.RawQuery = q.Encode()

	var payload map[string]any
	var lastErr error
	for attempt := 0; attempt < defaultRetries; attempt++ {
		payload = nil
		if err := c.doJSONRequest(ctx, "POST", &target, &payload); err != nil {
			lastErr = err
			if shouldRetry(err) {
				time.Sleep(backoff(attempt))
				continue
			}
			return nil, err
		}
		lastErr = nil
		break
	}
	if lastErr != nil {
		return nil, lastErr
	}

	config := firstString(payload, "config", "kubeconfig")
	if config == "" {
		return nil, fmt.Errorf("rancher kubeconfig response missing config")
	}
	return []byte(config), nil
}

func normalizeCluster(raw map[string]any) Cluster {
	return Cluster{
		ID:   firstString(raw, "id"),
		Name: firstString(raw, "name"),
	}
}

func joinClusterIDs(clusters []Cluster) string {
	parts := make([]string, 0, len(clusters))
	for _, cluster := range clusters {
		if cluster.ID != "" {
			parts = append(parts, cluster.ID)
		}
	}
	return strings.Join(parts, ", ")
}

func stripAPISuffix(path string) string {
	lower := strings.ToLower(path)
	if idx := strings.Index(lower, "/v1/"); idx != -1 {
		return path[:idx]
	}
	if idx := strings.Index(lower, "/v3/"); idx != -1 {
		return path[:idx]
	}
	if strings.HasSuffix(lower, "/v1") {
		return path[:len(path)-3]
	}
	if strings.HasSuffix(lower, "/v3") {
		return path[:len(path)-3]
	}
	return path
}
