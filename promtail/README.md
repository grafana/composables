# Promtail Composable

A Promtail log collector that ships container logs to Loki.

## What It Provides

- **Promtail**: Log collection agent that reads container logs and forwards them to Loki
- **Container Runtime Detection**: Automatically detects Docker vs Podman and configures appropriate volume mounts
- **Custom Config Support**: Mount your own promtail configuration file

## Services

| Service | Purpose | Ports |
|---------|---------|-------|
| `promtail` | Log collection and forwarding | 9080 (healthcheck only) |

## Container Runtime Support

The composable automatically detects whether you're running Docker or Podman:

- **Docker**: Mounts `/var/lib/docker/containers` (read-only) and `/var/run/docker.sock`
- **Podman**: Mounts `/run/podman/podman.sock` as the Docker socket

Detection happens at Tiltfile evaluation time by checking `docker --version` output.

## Exported Helper Functions

### `register_config(config_path)`

Mount a custom promtail configuration file. This replaces the default promtail config with your own.

**Parameters:**
- `config_path`: Absolute path to the promtail config YAML file

**Example:**
```python
promtail = cc.use('promtail')

def cc_export(cc):
    return cc.create(
        'my-app',
        os.path.dirname(__file__) + '/docker-compose.yaml',
        promtail,
        modifications=[
            promtail.register_config(
                os.path.dirname(__file__) + '/config/promtail-config.yaml'
            ),
        ],
    )
```

The config file is mounted to `/etc/promtail/config.yml` in the container.

## Wire-When Rules

### When `loki` is loaded
- Adds Loki as a `depends_on` for the promtail service
- Ensures Loki is started before promtail begins shipping logs

## Files

- `promtail.yaml` - Main compose file with promtail service
- `Tiltfile` - Composable definition with helper functions and runtime detection

## See Also

- [Loki Composable](../loki/) - Log storage backend
- [Grafana Composable](../grafana/) - Dashboard and log exploration
