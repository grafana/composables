# K3s API Server Composable

A lightweight Kubernetes API server for local development using K3s. This composable provides a minimal K3s instance with API aggregation support, CRD loading, and webhook routing capabilities.

## What It Provides

- **Kubernetes API Server**: Minimal K3s without scheduler/controllers
- **API Aggregation Support**: Full support for aggregated API servers (like Grafana's AppPlatform)
- **CRD Management**: Automatic loading of Custom Resource Definitions
- **Webhook Routing**: Traefik-based proxy for admission/validation webhooks
- **Certificate Generation**: Automatic TLS cert generation for API aggregation and webhooks
- **Kubeconfig Management**: Auto-generated kubeconfig with sync to local filesystem

## Services

| Service | Purpose | Ports |
|---------|---------|-------|
| `cert-provider` | Generates TLS certs for API aggregation and webhooks | - |
| `k3s-apiserver` | Minimal K3s API server | 6443 |
| `traefik` | Webhook proxy with network aliases for operators | 8080 (dashboard), 443 (webhooks) |
| `crd-loader` | Watches CRD directories and applies them to k3s | - |
| `kubeconfig-sync` | Syncs kubeconfig from container to host filesystem | - |

## Volumes

| Volume | Purpose | Mount Points |
|--------|---------|--------------|
| `k3s-data` | K3s persistent data | `/var/lib/rancher/k3s` |
| `k3s-certs` | API aggregation and webhook TLS certificates | `/certs` |
| `k3s-output` | Generated kubeconfig and runtime artifacts | `/output` |

## Exported Helper Functions

### `register_crds(crd_paths)`

Mount CRD directories/files into the crd-loader service for automatic application.

**Example:**
```python
k3s = cc_composable(
    name='k3s-apiserver',
    url='...',
    imports=['register_crds'],
)

def cc_export():
    return cc_create(
        'my-operator',
        os.path.dirname(__file__) + '/docker-compose.yaml',
        k3s,
        modifications=[
            k3s.register_crds(crd_paths=[
                os.path.dirname(__file__) + '/definitions',
                os.path.dirname(__file__) + '/crds',
            ]),
        ],
    )
```

### `register_webhook(operator_specs)`

Configure Traefik to route webhook requests to your operator services.

**Example:**
```python
k3s = cc_composable(
    name='k3s-apiserver',
    url='...',
    imports=['register_webhook'],
)

def cc_export():
    return cc_create(
        'my-operator',
        os.path.dirname(__file__) + '/docker-compose.yaml',
        k3s,
        modifications=[
            k3s.register_webhook(operator_specs=[
                {
                    'name': 'my-operator',
                    'namespace': 'default',
                    'port': 8443,
                },
            ]),
        ],
    )
```

### `get_kubeconfig_path()`

Returns the absolute path to the kubeconfig file on the host filesystem.

**Example:**
```python
k3s = cc_composable(
    name='k3s-apiserver',
    url='...',
    imports=['get_kubeconfig_path'],
)

if __file__ == config.main_path:
    kubeconfig = k3s.get_kubeconfig_path()
    print("Kubeconfig available at:", kubeconfig)
```

### `copy_kubeconfig_local(dest_path, resource_name='kubeconfig-copy')`

Creates a Tilt local_resource that waits for kubeconfig generation and copies it to a specified local path.

**Example:**
```python
k3s = cc_composable(
    name='k3s-apiserver',
    url='...',
    imports=['copy_kubeconfig_local'],
)

if __file__ == config.main_path:
    master = cc_generate_master_compose(cc_export(), cli_plugins)
    cc_docker_compose(master)

    # Copy kubeconfig for local kubectl usage
    k3s.copy_kubeconfig_local(
        dest_path=os.path.dirname(__file__) + '/generated-kubeconfig.yaml'
    )
```

**Why use this instead of `get_kubeconfig_path()`?**
- Handles timing: waits for k3s to generate the kubeconfig before copying
- Visible in Tilt UI: shows progress and completion status
- No race conditions: runs after Docker Compose starts, not during Tiltfile evaluation
- Per-project copies: each orchestrator gets its own local copy

## Wire-When Rules

This composable does not export wire-when rules. Instead, other composables (like Grafana) define how they wire to k3s-apiserver when it's present.

## Usage Examples

### Basic Usage

```python
# Load the k3s-apiserver composable
k3s = cc_composable(name='k3s-apiserver', url='https://github.com/grafana/composables')

def cc_export():
    return cc_create(
        'my-app',
        os.path.dirname(__file__) + '/docker-compose.yaml',
        k3s,  # k3s as a dependency
    )

if __file__ == config.main_path:
    master = cc_generate_master_compose(cc_export(), [])
    cc_docker_compose(master)
```

### With CRDs

```python
k3s = cc_composable(
    name='k3s-apiserver',
    url='https://github.com/grafana/composables',
    imports=['register_crds'],
)

def cc_export():
    return cc_create(
        'my-operator',
        os.path.dirname(__file__) + '/docker-compose.yaml',
        k3s,
        modifications=[
            k3s.register_crds(crd_paths=[
                os.path.dirname(__file__) + '/definitions',
            ]),
        ],
    )
```

### With Webhooks

```python
k3s = cc_composable(
    name='k3s-apiserver',
    url='https://github.com/grafana/composables',
    imports=['register_webhook'],
)

def cc_export():
    return cc_create(
        'my-operator',
        os.path.dirname(__file__) + '/docker-compose.yaml',
        k3s,
        modifications=[
            k3s.register_webhook(operator_specs=[
                {'name': 'my-operator', 'namespace': 'default', 'port': 8443},
            ]),
        ],
    )
```

### Complete Example

```python
# Full-featured operator with CRDs, webhooks, and local kubeconfig
k3s = cc_composable(
    name='k3s-apiserver',
    url='https://github.com/grafana/composables',
    imports=['register_crds', 'register_webhook', 'copy_kubeconfig_local'],
    labels=['k8s'],
)

def cc_export():
    return cc_create(
        'my-operator',
        os.path.dirname(__file__) + '/docker-compose.yaml',
        k3s,
        labels=['app'],
        modifications=[
            # Register CRDs
            k3s.register_crds(crd_paths=[
                os.path.dirname(__file__) + '/definitions',
            ]),

            # Configure webhook routing
            k3s.register_webhook(operator_specs=[
                {'name': 'my-operator', 'namespace': 'default', 'port': 8443},
            ]),
        ],
    )

if __file__ == config.main_path:
    master = cc_generate_master_compose(cc_export(), [])
    cc_docker_compose(master)

    # Copy kubeconfig for local kubectl access
    k3s.copy_kubeconfig_local(
        dest_path=os.path.dirname(__file__) + '/kubeconfig.yaml'
    )
```

## Using Kubectl Locally

After starting Tilt with `copy_kubeconfig_local()`:

```bash
# Set KUBECONFIG environment variable
export KUBECONFIG=/path/to/your-project/kubeconfig.yaml

# Use kubectl
kubectl get nodes
kubectl get crds
kubectl get all -A
```

Or use it inline:
```bash
kubectl --kubeconfig=/path/to/your-project/kubeconfig.yaml get nodes
```

## Architecture Details

### Certificate Generation

The `cert-provider` service generates three types of certificates:

1. **API Aggregation Certs** (`/certs/api-server-aggregation/`):
   - Client CA certificate for aggregated API servers
   - Client certificate for Grafana AppPlatform to authenticate with k3s

2. **Webhook TLS Certs** (`/certs/tls/`):
   - TLS certificates for webhook operators
   - Generated per operator specified in `WEBHOOK_OPERATORS`

### Webhook Routing

Traefik provides network aliases for webhook operators:

```
k8s API ───> Traefik (network alias: my-operator.default.svc) ───> my-operator:8443
```

This allows k3s to resolve webhook URLs like `https://my-operator.default.svc:443/validate` to the correct container.

### CRD Loading

The `crd-loader` service:
1. Waits for k3s API server to be ready
2. Watches mounted CRD directories
3. Applies all `.yaml`, `.yml`, and `.json` files as CRDs
4. Automatically applies CRDs from multiple plugins

## Troubleshooting

### Kubeconfig not appearing

Check that the `kubeconfig-sync` service is running:
```bash
docker compose ps kubeconfig-sync
```

View its logs:
```bash
docker compose logs kubeconfig-sync
```

### CRDs not loading

Check the `crd-loader` logs:
```bash
docker compose logs crd-loader
```

Verify your CRD paths are absolute paths in the Tiltfile:
```python
# Good (absolute path)
k3s.register_crds(crd_paths=[os.path.dirname(__file__) + '/definitions'])

# Bad (relative path)
k3s.register_crds(crd_paths=['./definitions'])
```

### Webhook not routing

1. Check that your operator is registered:
   ```bash
   docker compose logs traefik | grep "my-operator"
   ```

2. Verify the operator service name matches:
   ```yaml
   # docker-compose.yaml
   services:
     my-operator:  # This name must match register_webhook()
       image: my-operator:latest
       ports:
         - "8443:8443"
   ```

3. Check admission controller configuration includes the caBundle:
   ```yaml
   webhooks:
   - name: my-webhook
     clientConfig:
       caBundle: <base64-encoded-ca-cert>  # Must be present
       service:
         name: my-operator
         namespace: default
         port: 443
   ```

## Files

- `k3s-apiserver-generic.yaml` - Full K3s setup with all services (recommended)
- `k3s-apiserver-minimal.yaml` - Minimal K3s without webhook support (deprecated)
- `k3s-apiserver.yaml` - Symlink to generic (for backwards compatibility)
- `Tiltfile` - Composable definition with helper functions
- `output/` - Generated kubeconfig (created at runtime)

## Requirements

- Docker Compose with `include` directive support (v2.20+)
- Tilt v0.30+ (for compose_composer support)

## See Also

- [Compose Composer Documentation](../../tilt-extensions/compose_composer/README.md)
- [Service Model Example](../../service-model/) - Complete operator implementation
- [Grafana Composable](../grafana/) - How to wire Grafana to k3s-apiserver
