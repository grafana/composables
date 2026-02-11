# Prometheus Composable

A Prometheus metrics collection and storage server for development environments.

## What It Provides

- **Prometheus**: Full Prometheus instance with persistent storage
- **Scrape Config Generation**: Accumulating helpers generate a combined `prometheus.yml`
- **Remote Write**: Support for forwarding metrics to remote endpoints (e.g., Mimir)
- **Grafana Integration**: Auto-registers as a Grafana datasource when Grafana is present

## Services

| Service | Purpose | Ports |
|---------|---------|-------|
| `prometheus` | Metrics collection and storage | 9090 (configurable via `PROMETHEUS_PORT`) |

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PROMETHEUS_PORT` | `9090` | Host port to expose Prometheus |

## Volumes

| Volume | Purpose | Mount Points |
|--------|---------|--------------|
| `prometheus-data` | Persistent metrics storage | `/prometheus` |

## Exported Helper Functions

### `register_scrape_config(scrape_configs)`

Declare scrape targets for Prometheus. This is an **accumulating helper** — multiple composables can call it, and all scrape configs are collected by `process_accumulated_modifications()` to generate a combined `prometheus.yml`.

A base `prometheus` self-scrape job (localhost:9090) is always included automatically.

**Parameters:**
- `scrape_configs`: List of scrape config dicts, each with:
  - `job_name` (required): Prometheus job name
  - `static_configs` (optional): List of `{'targets': [...]}` dicts
  - `metrics_path` (optional): Metrics endpoint path (default: `/metrics`)
  - `scrape_interval` (optional): Per-job scrape interval (e.g., `10s`)
  - `scrape_timeout` (optional): Per-job scrape timeout (e.g., `5s`)

**Example:**
```python
prometheus = cc.use('prometheus')

def cc_export(cc):
    return cc.create(
        'my-app',
        os.path.dirname(__file__) + '/docker-compose.yaml',
        prometheus,
        modifications=[
            prometheus.register_scrape_config([
                {
                    'job_name': 'my-api',
                    'static_configs': [{'targets': ['api:8080']}],
                    'metrics_path': '/metrics',
                    'scrape_interval': '10s',
                    'scrape_timeout': '5s',
                },
                {
                    'job_name': 'envoy',
                    'static_configs': [{'targets': ['envoy:9901']}],
                    'metrics_path': '/stats/prometheus',
                },
            ]),
        ],
    )
```

### `register_remote_write(remote_write_specs)`

Declare remote_write targets for Prometheus. This is an **accumulating helper** — multiple composables can call it, and all targets are collected to generate the `remote_write:` section in `prometheus.yml`.

**Parameters:**
- `remote_write_specs`: List of remote_write config dicts, each with:
  - `url` (required): Remote write endpoint
  - `name` (optional): Name for logging/identification

**Example:**
```python
prometheus.register_remote_write([
    {'url': 'http://mimir:9009/api/v1/push', 'name': 'mimir'},
])
```

## Wire-When Rules

### When `grafana` is loaded
- Registers Prometheus as a Grafana datasource
- Adds `prometheus-datasource.yaml` to Grafana's provisioning directory

## Config Generation

When any composable uses `register_scrape_config()` or `register_remote_write()`, the `process_accumulated_modifications()` callback:

1. Collects all scrape configs and remote_write targets
2. Generates a complete `prometheus.yml` in `.cc/prometheus.yml`
3. Mounts it into the Prometheus container at `/etc/prometheus/prometheus.yml`

The generated config always includes:
- `global.scrape_interval: 15s` and `global.evaluation_interval: 15s`
- A self-scrape job for Prometheus itself (`localhost:9090`)
- All registered scrape configs and remote_write targets

## Files

- `prometheus.yaml` - Main compose file with Prometheus service
- `prometheus-datasource.yaml` - Grafana datasource provisioning
- `Tiltfile` - Composable definition with helper functions

## See Also

- [Grafana Composable](../grafana/) - Dashboard and visualization
- [Mimir Composable](../mimir/) - Long-term metrics storage (via remote_write)
