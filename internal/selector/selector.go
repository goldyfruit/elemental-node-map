package selector

import (
	"fmt"

	"k8s.io/apimachinery/pkg/labels"
)

// Parse parses a Kubernetes-style label selector.
func Parse(raw string) (labels.Selector, error) {
	if raw == "" {
		return labels.Everything(), nil
	}
	parsed, err := labels.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid label selector %q: %w", raw, err)
	}
	return parsed, nil
}
