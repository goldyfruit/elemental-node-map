# elemental-node-map

Match Rancher Elemental inventory hosts to downstream Kubernetes nodes with a clean terminal UI and machine-readable output.

## What this tool does

- Pulls Elemental inventory from Rancher.
- Pulls Kubernetes nodes from a downstream cluster (direct kubeconfig or via Rancher).
- Matches hosts to nodes using a deterministic strategy (machine ID, provider ID, IPs, hostname).
- Shows matched, ambiguous, and unmatched items with clear reasons.

## Terminology (important)

- **Elemental Host**: an inventory record in Rancher Elemental.
- **Rancher Machine**: the machine resource name (CAPI/Rancher).
- **K8s Node**: the Kubernetes node object in the downstream cluster.

One physical machine should map 1:1 across these views, but stale or missing inventory data can leave hosts or nodes unmatched.

## Installation

```bash
go build -o elemental-node-map ./
```

## Quick start

```bash
# Use Rancher to resolve the downstream cluster kubeconfig
export KUBECONFIG=~/.kube/rancher-local.yaml
./elemental-node-map match --rancher-cluster shared-mtl-001 --explain --wide
```

## Kubeconfig resolution

Precedence:
1. `--kubeconfig <path>`
2. `KUBECONFIG` (supports multiple paths)
3. `~/.kube/config`

Examples:

```bash
# kubeconfig from env
export KUBECONFIG=~/.kube/prod.yaml
./elemental-node-map match

# explicit kubeconfig overrides env
./elemental-node-map match --kubeconfig ~/.kube/dev.yaml --context dev
```

Verbose mode prints which kubeconfig/context were used:

```bash
./elemental-node-map nodes --verbose
```

## Rancher access

Provide the inventory collection URL and token:

```bash
export RANCHER_URL="https://rancher.example.com/v1/elemental.cattle.io.machineinventories"
export RANCHER_TOKEN="token-..."
export RANCHER_INSECURE_SKIP_TLS_VERIFY=true
```

Or via flags:

```bash
./elemental-node-map match \
  --rancher-url "https://rancher.example.com/v1/elemental.cattle.io.machineinventories" \
  --rancher-token "token-..." \
  --insecure-skip-tls-verify
```

If you use a Rancher-generated kubeconfig (local cluster), the CLI can derive Rancher URL/token automatically:

```bash
export KUBECONFIG=~/.kube/rancher-local.yaml
./elemental-node-map match --rancher-cluster shared-mtl-001
```

You can also set the downstream cluster by env:

```bash
export RANCHER_CLUSTER=shared-mtl-001
./elemental-node-map match
```

## Matching strategy

Order (first match wins, ambiguity preserved):

1. Machine ID / System UUID (exact)
2. Provider ID (exact)
3. Internal IP
4. External IP
5. Hostname (normalized)

When `--explain` is set, each match includes the reason and confidence score.

## Match examples

```bash
# basic match
./elemental-node-map match --rancher-cluster shared-mtl-001

# explain + wide (shows provider ID, machine ID, etc.)
./elemental-node-map match --rancher-cluster shared-mtl-001 --explain --wide

# show unmatched hosts/nodes
./elemental-node-map match --rancher-cluster shared-mtl-001 --show-unmatched

# filter nodes by Kubernetes label selectors
./elemental-node-map match --selector 'env=prod,node-role.kubernetes.io/worker'

# filter nodes by label text (matches label key or value)
./elemental-node-map match --labels 5090
./elemental-node-map match --labels 5090,5070
./elemental-node-map match --labels 'machine.cattle.io/*'
./elemental-node-map match --labels '/machine\.cattle\.io\/.*/'
```

## Node listing and labels

```bash
# list nodes
./elemental-node-map nodes

# show label keys as columns (exact keys)
./elemental-node-map nodes --label-keys 'topology.kubernetes.io/zone,env'

# show label keys with wildcard or regex patterns
./elemental-node-map nodes --label-keys 'topology.kubernetes.io/*'
./elemental-node-map nodes --label-keys '/topology\\.kubernetes\\.io\\/.*/'

# show all labels in a single column
./elemental-node-map nodes --labels

# label explorer
./elemental-node-map labels keys
./elemental-node-map labels values env
```

## Output formats

```bash
./elemental-node-map match --output json
./elemental-node-map nodes --output yaml
```

## Caching (performance)

When `--rancher-cluster` is used, the downstream kubeconfig fetched from Rancher is cached for 10 minutes in your user cache directory (e.g. `~/.cache/elemental-node-map/`). This makes repeated runs much faster.

## Exit codes

- `0` success
- `1` usage/config error
- `2` API/auth error
- `3` partial results (ambiguous matches)

## Troubleshooting

- Use `--verbose` to see which kubeconfig/context is selected and whether cache is used.
- If you see unmatched hosts with no identifiers, the inventory record is missing key fields (machine name, hostname, IDs).

## Development

```bash
go test ./...
```
