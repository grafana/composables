# Grafana Composable

A Grafana development environment with plugin support, dashboard provisioning, and API server aggregation capabilities.

## What It Provides

- **Grafana Enterprise**: Full Grafana instance with development mode enabled
- **Plugin Support**: Mount and provision development plugins
- **Dashboard Provisioning**: Register dashboard directories
- **API Aggregation**: Integration with k3s-apiserver for Kubernetes APIs
- **Database Support**: Auto-wires to MySQL when present
- **User Provisioning**: Create users with org roles and RBAC assignments via API

## Services

| Service | Purpose | Ports |
|---------|---------|-------|
| `grafana` | Main Grafana instance | 3000 (configurable via `GRAFANA_PORT`) |
| `grafana-debug` | Debug container with read-only access to volumes | - |

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `GRAFANA_IMAGE` | `grafana/grafana-enterprise` | Docker image name |
| `GRAFANA_VERSION` | `latest` | Docker image tag/version |
| `GRAFANA_PORT` | `3000` | Host port to expose Grafana |

### Image Override Examples

```bash
# Use a specific version
GRAFANA_VERSION=11.0.0 tilt up

# Use community edition
GRAFANA_IMAGE=grafana/grafana GRAFANA_VERSION=11.0.0 tilt up

# Use a local/custom image
GRAFANA_IMAGE=my-registry/custom-grafana GRAFANA_VERSION=dev tilt up
```

## Volumes

| Volume | Purpose | Mount Points |
|--------|---------|--------------|
| `grafana-data` | Persistent Grafana data | `/var/lib/grafana` |

## Exported Helper Functions

### `register_plugin(plugin_specs)`

Register Grafana plugins with automatic provisioning file generation.

**Parameters:**
- `plugin_specs`: List of dicts, each with:
  - `name` (required): Plugin ID (e.g., `grafana-irm-app`)
  - `dist` (required): Plugin dist path or volume mount spec
  - `provisioning_file` (optional): Custom provisioning YAML path
  - `depends_on` (optional): Service dependencies
  - `feature_toggles` (optional): Comma-separated feature toggles
  - `profiles` (optional): Only register if profile is active

**Example:**
```python
grafana = cc_import(name='grafana', url='...')

def cc_export():
    return cc_create(
        'my-app',
        os.path.dirname(__file__) + '/docker-compose.yaml',
        grafana,
        modifications=[
            grafana.register_plugin([{
                'name': 'grafana-irm-app',
                'dist': os.path.dirname(__file__) + '/dist',
                'feature_toggles': 'myFeature,anotherFeature',
            }]),
        ],
    )
```

### `register_dashboards(dashboards_path)`

Mount a dashboard provisioning directory.

**Parameters:**
- `dashboards_path`: Absolute path to dashboard provisioning directory

**Example:**
```python
grafana.register_dashboards(os.path.dirname(__file__) + '/provisioning/dashboards')
```

**Directory Structure:**
```
provisioning/dashboards/
├── dashboards.yml              # Provider configuration
├── dashboards-monitoring/
│   └── overview.json
└── dashboards-alerts/
    └── alerts.json
```

### `provision_plugins(plugin_paths)`

Mount plugin provisioning files directly (for advanced use cases).

**Parameters:**
- `plugin_paths`: List with ONE directory path (Grafana limitation)

**Example:**
```python
grafana.provision_plugins(['/path/to/provisioning/plugins'])
```

### `configure_data_volume(enabled=True)`

Configure Grafana's data persistence.

**Parameters:**
- `enabled`: If `False`, uses tmpfs for ephemeral storage

**Example:**
```python
# Fresh state on every restart (useful for testing)
grafana.configure_data_volume(enabled=False)
```

### `register_aggregator_config(api_groups)`

Register API groups for Grafana AppPlatform aggregation (requires k3s-apiserver).

**Parameters:**
- `api_groups`: List of dicts, each with:
  - `group` (required): API group (e.g., `servicemodel.ext.grafana.com`)
  - `version` (required): API version (e.g., `v1alpha1`)
  - `host` (optional): API server hostname (default: `k3s-apiserver`)
  - `port` (optional): API server port (default: `6443`)

**Example:**
```python
grafana.register_aggregator_config(api_groups=[
    {
        'group': 'servicemodel.ext.grafana.com',
        'version': 'v1alpha1',
    },
])
```

### `provision_users(user_specs)`

Declare users to provision via the Grafana HTTP API after startup. Multiple composables can call this helper — all user specs are accumulated and provisioned by a single `grafana-user-setup` service.

**Parameters:**
- `user_specs`: List of dicts, each with:
  - `username` (required): Login name
  - `password` (required): User password
  - `role` (optional): Org role — `Viewer`, `Editor`, or `Admin` (default: `Viewer`)
  - `rbac_roles` (optional): List of fine-grained RBAC role names (requires Grafana Enterprise)
  - `email` (optional): Email address
  - `name` (optional): Display name

**Example:**
```python
grafana = cc.use('grafana')

def cc_export(cc):
    return cc.create(
        'my-app',
        os.path.dirname(__file__) + '/docker-compose.yaml',
        grafana,
        modifications=[
            grafana.provision_users(user_specs=[
                {
                    'username': 'dev-user',
                    'password': 'dev-pass',
                    'role': 'Editor',
                },
                {
                    'username': 'viewer',
                    'password': 'viewer-pass',
                    'role': 'Viewer',
                    'rbac_roles': ['fixed:dashboards:reader'],
                },
            ]),
        ],
    )
```

**How it works:**
- Creates a `grafana-user-setup` service that runs after Grafana is healthy
- Uses the Grafana admin API with basic auth to create users and assign roles
- Idempotent: skips creation if a user already exists, still updates roles

### `configure_admin(username, password)`

Set admin credentials used by the user provisioning service for API authentication. Also sets `GF_SECURITY_ADMIN_USER` and `GF_SECURITY_ADMIN_PASSWORD` on the Grafana service.

**Parameters:**
- `username`: Admin username (default: `admin`)
- `password`: Admin password (default: `admin`)

If not called, the provisioning service defaults to `admin`/`admin`.

**Example:**
```python
modifications=[
    grafana.configure_admin(username='myadmin', password='secret'),
    grafana.provision_users(user_specs=[...]),
]
```

## Wire-When Rules

This composable automatically wires to other composables when present:

### When `mysql` is loaded
- Configures Grafana to use MySQL as database
- Sets session provider to MySQL

### When `k3s-apiserver` is loaded
- Mounts k3s certificates for API aggregation
- Configures kubeconfig and proxy client certificates
- Creates `aggregator-config` service

### When `nats` is loaded
- Configures NATS server address for real-time events

### When `renderer` is loaded
- Configures Grafana image rendering service

### When `jaeger` is loaded
- Configures OpenTelemetry tracing export

## Usage Examples

### Basic Usage

```python
grafana = cc_import(name='grafana', url='https://github.com/grafana/composables')

def cc_export():
    return cc_create(
        'my-app',
        os.path.dirname(__file__) + '/docker-compose.yaml',
        grafana,
    )
```

### With Plugin Development

```python
grafana = cc_import(name='grafana', url='...')

def cc_export():
    return cc_create(
        'my-plugin',
        os.path.dirname(__file__) + '/docker-compose.yaml',
        grafana,
        modifications=[
            grafana.register_plugin([{
                'name': 'my-grafana-plugin',
                'dist': os.path.dirname(__file__) + '/packages/plugin/dist',
                'depends_on': ['plugin-build'],
                'feature_toggles': 'myPluginFeature',
            }]),
            grafana.register_dashboards(
                os.path.dirname(__file__) + '/provisioning/dashboards'
            ),
        ],
    )
```

### With k3s Integration

```python
grafana = cc_import(name='grafana', url='...')
k3s = cc_import(name='k3s-apiserver', url='...')

def cc_export():
    return cc_create(
        'my-operator',
        os.path.dirname(__file__) + '/docker-compose.yaml',
        grafana,
        k3s,
        modifications=[
            grafana.register_aggregator_config(api_groups=[
                {'group': 'myapp.grafana.com', 'version': 'v1'},
            ]),
        ],
    )
```

### With User Provisioning

```python
grafana = cc.use('grafana')

def cc_export(cc):
    return cc.create(
        'my-app',
        os.path.dirname(__file__) + '/docker-compose.yaml',
        grafana,
        modifications=[
            grafana.configure_admin(username='admin', password='grafana'),
            grafana.provision_users(user_specs=[
                {
                    'username': 'developer',
                    'password': 'dev123',
                    'role': 'Editor',
                    'email': 'dev@example.com',
                },
                {
                    'username': 'readonly',
                    'password': 'view123',
                    'role': 'Viewer',
                    'rbac_roles': ['fixed:dashboards:reader'],
                },
            ]),
        ],
    )
```

### Profile-Based Plugin Loading

```python
grafana.register_plugin([
    {
        'name': 'grafana-irm-app',
        'dist': '/path/to/irm/dist',
        'profiles': ['irm'],  # Only loads with CC_PROFILES=irm
    },
    {
        'name': 'grafana-oncall-app',
        'dist': '/path/to/oncall/dist',
        'profiles': ['oncall'],  # Only loads with CC_PROFILES=oncall
    },
])
```

## Troubleshooting

### Plugin not loading

1. Check that the plugin dist directory exists and contains `plugin.json`
2. Verify the plugin appears in Grafana's unsigned plugins list:
   ```bash
   docker compose exec grafana env | grep GF_PLUGINS
   ```
3. Check Grafana logs for plugin errors:
   ```bash
   docker compose logs grafana | grep -i plugin
   ```

### Dashboard not appearing

1. Verify the dashboards path is absolute:
   ```python
   # Good
   grafana.register_dashboards(os.path.dirname(__file__) + '/dashboards')
   # Bad
   grafana.register_dashboards('./dashboards')
   ```
2. Check that `dashboards.yml` provider config exists and references correct paths

### Database connection issues

Check that MySQL composable is loaded and healthy:
```bash
docker compose ps db
docker compose logs grafana | grep -i database
```

## Files

- `grafana.yaml` - Main compose file with Grafana and debug services
- `Tiltfile` - Composable definition with helper functions

## Requirements

- Docker Compose with `include` directive support (v2.20+)
- Tilt v0.30+ (for compose_composer support)

## See Also

- [k3s-apiserver Composable](../k3s-apiserver/) - Kubernetes API server for aggregation
- [MySQL Composable](../mysql/) - Database backend for Grafana
