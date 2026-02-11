# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Prerequisites

Before making any changes, read the compose_composer framework documentation at:
https://github.com/grafana/tilt-extensions/tree/main/compose_composer

Clone a local copy if you do not have access to one. It has its own `CLAUDE.md` that explains the full API, processing pipeline, and module design.

## What This Is

This repository contains **composables** — modular Tilt extensions that wrap Docker Compose services with helper functions, declarative wiring rules, and configuration accumulation. They plug into the `compose_composer` framework to enable dynamic assembly of development environments.

## Commands

```bash
# Run ALL composable tests (from this directory)
make test

# Run a single composable's tests
make -C grafana test
# Or directly:
cd grafana/test && tilt ci

# Run a specific integration test
cd grafana/test && tilt ci -f Tiltfile.integration
cd grafana/test && tilt ci -f Tiltfile.integration-users

# Start services interactively (for manual testing)
cd grafana/test && tilt up -f Tiltfile.integration

# List all composables
make list-components
```

## Language

All code is **Starlark** (not Python). Key differences:
- No `import`, use `load()`; no classes, use `struct()`
- Type checking: `type(x) != 'string'` (not `isinstance`)
- No `try/except` — validate inputs and call `fail()` on errors
- String formatting: `%` operator and `.format()` both work
- Shell scripts in compose commands need `$$` for literal dollar signs (compose interpolation)

## Architecture

### Composable Callbacks

Every composable implements callbacks that compose_composer discovers and invokes:

**Required — `cc_export(cc)`**: Returns plugin struct. This is the composable's identity.
```python
def cc_export(cc):
    return cc.create('redis', os.path.dirname(__file__) + '/redis.yaml', labels=['infra'])
```

**Optional — `cc_wire_when(cc)`**: Declarative rules for automatic inter-service wiring. When a named dependency is loaded alongside this composable, the specified modifications apply. This is symmetric — the result is the same regardless of which composable is the orchestrator.
```python
def cc_wire_when(cc):
    return {
        'grafana': {  # When grafana is also loaded...
            'services': {
                'grafana': {  # ...modify the grafana service
                    'depends_on': ['prometheus'],
                    'volumes': ['./datasource.yaml:/etc/grafana/provisioning/datasources/prom.yaml:ro'],
                },
            },
        },
    }
```

**Optional — `process_accumulated_modifications(all_modifications, orchestrator_dir)`**: Collects marker dicts from all plugins and generates services/configs. Used by grafana, mysql, prometheus, mimir.

**Optional — Helper functions**: Take `cc` as first arg (injected by compose_composer), return dicts with `services` key. Called via `modifications=` parameter on `cc.create()`.

### Accumulation Pattern

The key architectural pattern for cross-composable configuration. Flow:

1. Plugin A calls `grafana.provision_users([...])` → returns `{'_grafana_user_specs': [...], '_target': 'grafana'}`
2. Plugin B calls `grafana.provision_users([...])` → returns another marker
3. Both markers go into `modifications=` on their respective `cc.create()` calls
4. compose_composer collects all markers, calls `grafana.process_accumulated_modifications(all_markers, dir)`
5. Grafana deduplicates, generates a single setup service with all users

Composables using this pattern: **grafana** (users, aggregator config), **mysql** (database creation), **prometheus** (scrape configs, remote_write), **mimir** (datasource config).

### Deep Merge Semantics

When compose_overrides are applied, compose_composer deep-merges:
- Dicts merge recursively
- Lists concatenate (avoiding duplicates)
- Special env vars (`GF_FEATURE_TOGGLES_ENABLE`, `GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS`, `WEBHOOK_OPERATORS`, `API_GROUPS`) concatenate with commas
- URLs (containing `://`) always replace, never concatenate
- Scalars: last wins

## Writing a Composable

### Minimal (redis-style)
Just `cc_export` returning `cc.create()` + a docker-compose YAML file.

### With Helpers (prometheus-style)
Add helper functions that return marker dicts. Add `process_accumulated_modifications()` to collect and process all markers into config files or setup services.

### With Wiring (grafana-style)
Add `cc_wire_when()` to declare modifications that apply when specific other composables are loaded. Add helpers for plugins, dashboards, users, etc.

### File Layout
```
composable-name/
├── Tiltfile                    # Callbacks + helpers
├── composable-name.yaml        # Docker Compose service definition
├── Makefile                    # Test runner (delegates to test/)
└── test/
    ├── Tiltfile                # Unit tests (copy functions, run assertions)
    ├── Tiltfile.integration    # Integration tests (use compose_composer pipeline)
    ├── docker-compose.yaml     # Minimal compose for test orchestrator
    └── Makefile
```

## Testing Pattern

Unit tests **copy functions** from the parent Tiltfile into the test file (removing the `cc` parameter) and test them in isolation. This is the established pattern — do not try to import from the parent.

```python
# In test/Tiltfile — function copied WITHOUT cc param
def provision_users(user_specs):
    # ... same body as parent, minus cc parameter ...

def test_provision_users_returns_marker():
    result = provision_users(user_specs=[{'username': 'test', 'password': 'pass'}])
    assert_true('_grafana_user_specs' in result, "Should have marker")
    print("  [PASS] test_provision_users_returns_marker")
```

Integration tests load compose_composer, run the full pipeline, and verify staged `.cc/` files or live services.

## CI

GitHub Actions workflow at `.github/workflows/test.yml` runs `make test` on every PR and push to main. Each composable's Makefile runs `tilt ci -f Tiltfile.integration` in its test directory. Environment overrides loaded from `.env.docker-compose` (e.g., custom ports to avoid conflicts).
