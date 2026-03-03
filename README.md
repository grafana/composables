# Composables

Reusable Tilt extensions for common development services. Each composable provides a service with helper functions, wire-when rules, and docker-compose configurations.

> **Note:** This project is under active development. APIs and composable interfaces may change.

## Available Components

- **azurite** - Azure Storage emulator
- **bigquery-emulator** - BigQuery emulator
- **clickhouse** - ClickHouse database
- **grafana** - Grafana with API server aggregation support
- **jaeger** - Jaeger tracing
- **k3s-apiserver** - Lightweight Kubernetes API server
- **loki** - Loki log aggregation
- **mssql-test** - Microsoft SQL Server for testing
- **mysql** - MySQL database
- **mysql-test** - MySQL for testing
- **nats** - NATS messaging
- **postgres-test** - PostgreSQL for testing
- **prometheus** - Prometheus monitoring
- **promtail** - Promtail log collector
- **rabbitmq** - RabbitMQ message broker
- **redis** - Redis cache
- **renderer** - Image rendering service
- **tempo** - Tempo tracing

## Usage

Each composable exports a Tilt extension that can be loaded in your Tiltfile:

```python
load('ext://compose_composer', 'cc_import')

# Import composables
grafana = cc_import(name='grafana', url='https://github.com/grafana/composables@main')
mysql = cc_import(name='mysql', url='https://github.com/grafana/composables@main')

```

## Testing

Run all composable tests:

```bash
make test
```

List available components:

```bash
make list-components
```

## Structure

Each composable directory contains:
- `Tiltfile` - Extension code with helper functions
- `*.yaml` - Docker Compose service definitions
- `Makefile` - Test runner
- `test/` - Unit and integration tests

## Environment

Configuration is loaded from `.env.docker-compose` if present. This attempts to reduce port conflicts for `make test` if other systems are running.

## Contributing

See our [contributing guide](CONTRIBUTING.md).

## License

[Apache License 2.0](LICENSE)
