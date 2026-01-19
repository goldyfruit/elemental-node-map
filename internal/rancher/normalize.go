package rancher

import (
	"fmt"
	"strings"

	"github.com/goldyfruit/elemental-node-mapper/internal/types"
)

func normalizeHost(raw map[string]any) types.InventoryHost {
	host := types.InventoryHost{}
	host.ID = firstString(raw, "id", "metadata.name", "name", "metadata.generateName")
	host.UID = firstString(raw, "metadata.uid", "uid")
	host.Namespace = firstString(raw, "metadata.namespace", "namespace")
	host.Labels = mergeLabels(
		firstStringMap(raw, "metadata.labels"),
		firstStringMap(raw, "spec.labels"),
		firstStringMap(raw, "labels"),
	)
	host.Metadata = firstStringMap(raw, "metadata.annotations")
	host.MachineName = firstString(
		raw,
		"spec.machineName",
		"status.machineName",
		"machineName",
		"spec.machine.name",
		"status.machine.name",
		"spec.machineRef.name",
		"status.machineRef.name",
		"spec.machine",
		"status.machine",
		"spec.inventory.machineName",
		"status.inventory.machineName",
		"spec.machineInventory.machineName",
		"status.machineInventory.machineName",
	)
	if host.MachineName != "" {
		host.MachineName = normalizeMachineName(host.MachineName)
	}
	if host.MachineName == "" {
		host.MachineName = firstLabelValue(host.Labels,
			"elemental.cattle.io/machine-name",
			"elemental.cattle.io/machine",
			"machine.cattle.io/name",
			"machine.cattle.io/machine",
			"machine.cattle.io/machine-name",
			"cluster.x-k8s.io/machine",
			"cattle.io/machine",
			"cattle.io/machine-name",
			"rke.cattle.io/machine",
			"management.cattle.io/machine",
			"provisioning.cattle.io/machine",
			"fleet.cattle.io/machine",
			"machine-name",
			"machine",
		)
		if host.MachineName != "" {
			host.MachineName = normalizeMachineName(host.MachineName)
		}
	}
	if host.MachineName == "" {
		host.MachineName = firstLabelValue(host.Metadata,
			"elemental.cattle.io/machine-name",
			"elemental.cattle.io/machine",
			"machine.cattle.io/name",
			"machine.cattle.io/machine",
			"machine.cattle.io/machine-name",
			"cluster.x-k8s.io/machine",
			"cattle.io/machine",
			"cattle.io/machine-name",
			"rke.cattle.io/machine",
			"management.cattle.io/machine",
			"provisioning.cattle.io/machine",
			"fleet.cattle.io/machine",
			"machine-name",
			"machine",
		)
		if host.MachineName != "" {
			host.MachineName = normalizeMachineName(host.MachineName)
		}
	}
	host.Hostname = firstString(
		raw,
		"spec.nodeName",
		"status.nodeName",
		"status.hostname",
		"status.hostName",
		"status.nodeRef.name",
		"spec.nodeRef.name",
		"spec.hostname",
		"spec.inventory.hostname",
		"status.inventory.hostname",
		"spec.machineInventory.hostname",
		"status.machineInventory.hostname",
		"hostname",
		"name",
		"metadata.name",
	)
	host.MachineID = firstString(
		raw,
		"spec.machineID",
		"spec.machineId",
		"machineID",
		"machineId",
		"status.machineID",
		"status.machineId",
		"spec.inventory.machineID",
		"spec.inventory.machineId",
		"status.inventory.machineID",
		"status.inventory.machineId",
		"spec.machineInventory.machineID",
		"spec.machineInventory.machineId",
		"status.machineInventory.machineID",
		"status.machineInventory.machineId",
	)
	host.SystemUUID = firstString(
		raw,
		"spec.systemUUID",
		"spec.systemUuid",
		"systemUUID",
		"systemUuid",
		"status.systemUUID",
		"status.systemUuid",
		"spec.inventory.systemUUID",
		"spec.inventory.systemUuid",
		"status.inventory.systemUUID",
		"status.inventory.systemUuid",
		"spec.machineInventory.systemUUID",
		"spec.machineInventory.systemUuid",
		"status.machineInventory.systemUUID",
		"status.machineInventory.systemUuid",
	)
	host.ProviderID = firstString(
		raw,
		"spec.providerID",
		"spec.providerId",
		"providerID",
		"providerId",
		"status.providerID",
		"status.providerId",
		"spec.inventory.providerID",
		"spec.inventory.providerId",
		"status.inventory.providerID",
		"status.inventory.providerId",
		"spec.machineInventory.providerID",
		"spec.machineInventory.providerId",
		"status.machineInventory.providerID",
		"status.machineInventory.providerId",
	)
	host.IPs = firstIPSlice(
		raw,
		"spec.ipAddresses",
		"spec.ipAddress",
		"spec.addresses",
		"ipAddresses",
		"ipAddress",
		"status.ipAddresses",
		"status.ipAddress",
		"status.addresses",
		"status.nodeAddresses",
		"status.node.addresses",
		"spec.inventory.ipAddresses",
		"spec.inventory.ipAddress",
		"status.inventory.ipAddresses",
		"status.inventory.ipAddress",
		"spec.machineInventory.ipAddresses",
		"spec.machineInventory.ipAddress",
		"status.machineInventory.ipAddresses",
		"status.machineInventory.ipAddress",
	)

	if host.ID == "" {
		host.ID = idFromLink(firstString(raw, "links.self", "links.selfLink", "links.view", "links.update"))
	}

	if host.ID == "" {
		host.ID = host.Hostname
	}
	return host
}

func mergeLabels(maps ...map[string]string) map[string]string {
	merged := map[string]string{}
	for _, m := range maps {
		for key, value := range m {
			merged[key] = value
		}
	}
	if len(merged) == 0 {
		return nil
	}
	return merged
}

func firstString(raw map[string]any, paths ...string) string {
	for _, path := range paths {
		if value, ok := getString(raw, path); ok {
			return value
		}
	}
	return ""
}

func firstStringSlice(raw map[string]any, paths ...string) []string {
	for _, path := range paths {
		if value, ok := getStringSlice(raw, path); ok {
			return value
		}
	}
	return nil
}

func firstIPSlice(raw map[string]any, paths ...string) []string {
	for _, path := range paths {
		if value, ok := getIPSlice(raw, path); ok {
			return value
		}
	}
	return nil
}

func firstStringMap(raw map[string]any, paths ...string) map[string]string {
	for _, path := range paths {
		if value, ok := getStringMap(raw, path); ok {
			return value
		}
	}
	return nil
}

func getString(raw map[string]any, path string) (string, bool) {
	value, ok := getValue(raw, path)
	if !ok {
		return "", false
	}
	switch typed := value.(type) {
	case string:
		return typed, typed != ""
	default:
		return fmt.Sprintf("%v", typed), true
	}
}

func getStringSlice(raw map[string]any, path string) ([]string, bool) {
	value, ok := getValue(raw, path)
	if !ok {
		return nil, false
	}
	switch typed := value.(type) {
	case []string:
		return filterStrings(typed), true
	case []any:
		var out []string
		for _, item := range typed {
			out = append(out, fmt.Sprintf("%v", item))
		}
		return filterStrings(out), true
	case string:
		if typed == "" {
			return nil, false
		}
		return []string{typed}, true
	default:
		return []string{fmt.Sprintf("%v", typed)}, true
	}
}

func getIPSlice(raw map[string]any, path string) ([]string, bool) {
	value, ok := getValue(raw, path)
	if !ok {
		return nil, false
	}
	ips := extractIPStrings(value)
	ips = filterStrings(ips)
	if len(ips) == 0 {
		return nil, false
	}
	return ips, true
}

func extractIPStrings(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		var out []string
		for _, item := range typed {
			out = append(out, extractIPStrings(item)...)
		}
		return out
	case map[string]any:
		return extractIPStringsFromMap(typed)
	case map[string]string:
		return extractIPStringsFromStringMap(typed)
	case string:
		if typed == "" {
			return nil
		}
		return []string{typed}
	default:
		return nil
	}
}

func extractIPStringsFromMap(value map[string]any) []string {
	for _, key := range []string{"address", "ip", "ipAddress", "value"} {
		if raw, ok := value[key]; ok {
			return []string{fmt.Sprintf("%v", raw)}
		}
	}
	return nil
}

func extractIPStringsFromStringMap(value map[string]string) []string {
	for _, key := range []string{"address", "ip", "ipAddress", "value"} {
		if raw, ok := value[key]; ok {
			return []string{raw}
		}
	}
	return nil
}

func firstLabelValue(labels map[string]string, keys ...string) string {
	for _, key := range keys {
		if value, ok := labels[key]; ok {
			trimmed := strings.TrimSpace(value)
			if trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func normalizeMachineName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if idx := strings.LastIndex(value, "/"); idx >= 0 && idx < len(value)-1 {
		return strings.TrimSpace(value[idx+1:])
	}
	if idx := strings.LastIndex(value, ":"); idx >= 0 && idx < len(value)-1 {
		return strings.TrimSpace(value[idx+1:])
	}
	return value
}

func idFromLink(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if idx := strings.Index(value, "?"); idx >= 0 {
		value = value[:idx]
	}
	if idx := strings.Index(value, "#"); idx >= 0 {
		value = value[:idx]
	}
	if idx := strings.LastIndex(value, "/"); idx >= 0 && idx < len(value)-1 {
		value = value[idx+1:]
	}
	return strings.TrimSpace(value)
}

func getStringMap(raw map[string]any, path string) (map[string]string, bool) {
	value, ok := getValue(raw, path)
	if !ok {
		return nil, false
	}
	switch typed := value.(type) {
	case map[string]string:
		return typed, len(typed) > 0
	case map[string]any:
		out := map[string]string{}
		for key, val := range typed {
			out[key] = fmt.Sprintf("%v", val)
		}
		if len(out) == 0 {
			return nil, false
		}
		return out, true
	default:
		return nil, false
	}
}

func getValue(raw map[string]any, path string) (any, bool) {
	parts := strings.Split(path, ".")
	var current any = raw
	for _, part := range parts {
		asMap, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = asMap[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func filterStrings(values []string) []string {
	var out []string
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
