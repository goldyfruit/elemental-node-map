package rancher

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

type Machine struct {
	ID          string
	Name        string
	ClusterName string
	NodeName    string
	ProviderID  string
	Labels      map[string]string
	Annotations map[string]string
}

func MachinesURLFromInventoryURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid rancher URL: %w", err)
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	basePath := strings.TrimSuffix(stripAPISuffix(parsed.Path), "/")
	if basePath == "" {
		parsed.Path = "/v1/cluster.x-k8s.io.machine"
	} else {
		parsed.Path = basePath + "/v1/cluster.x-k8s.io.machine"
	}
	return parsed, nil
}

func (c *Client) ListMachines(ctx context.Context) ([]Machine, error) {
	var machines []Machine
	nextURL := c.withLimit(c.baseURL, 200)

	for nextURL != nil {
		page, err := c.fetchPage(ctx, nextURL)
		if err != nil {
			return nil, err
		}
		for _, raw := range page.Data {
			machines = append(machines, normalizeMachine(raw))
		}
		nextURL = page.NextURL(c.baseURL)
	}

	return machines, nil
}

func MachineNameMap(machines []Machine, clusterName string) map[string]string {
	mapByNode := map[string]string{}
	for _, machine := range machines {
		if clusterName != "" && machine.ClusterName != "" && machine.ClusterName != clusterName {
			continue
		}
		if machine.NodeName == "" || machine.Name == "" {
			continue
		}
		mapByNode[machine.NodeName] = machine.Name
	}
	return mapByNode
}

func normalizeMachine(raw map[string]any) Machine {
	machine := Machine{}
	machine.ID = firstString(raw, "id", "metadata.name")
	machine.Name = firstString(raw, "metadata.name", "name", "id", "spec.machineName", "spec.machine.name")
	machine.ClusterName = firstString(
		raw,
		"spec.clusterName",
		"status.clusterName",
		"metadata.labels.cluster.x-k8s.io/cluster-name",
		"metadata.labels.provisioning.cattle.io/cluster-name",
		"metadata.labels.cluster-name",
		"metadata.labels.cluster",
	)
	machine.NodeName = firstString(
		raw,
		"status.nodeRef.name",
		"status.nodeName",
		"spec.nodeRef.name",
		"spec.nodeName",
		"status.node",
	)
	machine.ProviderID = firstString(raw, "spec.providerID", "status.providerID")
	machine.Labels = mergeLabels(firstStringMap(raw, "metadata.labels"))
	machine.Annotations = firstStringMap(raw, "metadata.annotations")

	machine.Name = normalizeMachineName(machine.Name)
	machine.NodeName = normalizeMachineName(machine.NodeName)
	return machine
}
