# Contributing Guidelines

This document is a guide to help you through the process of contributing to composables.

## Getting Started

Composables are modular [Tilt](https://tilt.dev/) extensions that wrap Docker Compose services. They plug into the [compose_composer](https://github.com/grafana/tilt-extensions/tree/main/compose_composer) framework to enable dynamic assembly of development environments.

### Prerequisites

- [Tilt](https://docs.tilt.dev/install.html)
- [Docker](https://docs.docker.com/get-docker/) (with Compose)
- Make

### Running Tests

```bash
# Run all composable tests
make test

# Run a single composable's tests
make -C grafana test

# Run a specific integration test
cd grafana/test && tilt ci -f Tiltfile.integration
```

## How to Contribute

1. Fork the repository and create your branch from `main`.
2. Make your changes, following the conventions below.
3. Add or update tests for your changes.
4. Ensure `make test` passes.
5. Open a pull request.

## Writing a Composable

Each composable lives in its own directory with this layout:

```
composable-name/
├── Tiltfile                    # Callbacks + helpers
├── composable-name.yaml        # Docker Compose service definition
├── Makefile                    # Test runner
└── test/
    ├── Tiltfile                # Unit tests
    ├── Tiltfile.integration    # Integration tests
    ├── docker-compose.yaml     # Minimal compose for test orchestrator
    └── Makefile
```

At minimum, a composable must implement `cc_export(cc)` returning `cc.create()`. See the existing composables (e.g., `redis/`) for a minimal example or `grafana/` for a full-featured one.

## Code Style

All code is **Starlark** (not Python). Key conventions:

- Use `load()` instead of `import`; use `struct()` instead of classes.
- Type checking: `type(x) != 'string'` (not `isinstance`).
- No `try/except` — validate inputs and call `fail()` on errors.
- Use `$$` for literal dollar signs in shell commands within compose files.
- Always use **dict format** for `depends_on`, never list format:

```python
# Correct
'depends_on': {'redis': {'condition': 'service_started'}}

# Wrong — causes merge conflicts
'depends_on': ['redis']
```

## Testing Pattern

Unit tests **copy functions** from the parent Tiltfile into the test file (removing the `cc` parameter) and test them in isolation. This is the established pattern — do not try to import from the parent.

Integration tests load compose_composer, run the full pipeline, and verify staged files or live services.

## Reporting Issues

See our [issue templates](https://github.com/grafana/composables/issues/new/choose) or join the [discussions](https://github.com/grafana/composables/discussions).

## License

By contributing, you agree that your contributions will be licensed under the [Apache License 2.0](LICENSE).
