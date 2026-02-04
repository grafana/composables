# Grafana Composable Tests

Comprehensive unit tests for the grafana composable helper functions.

## Running Tests

### Unit Tests

```bash
cd composables/grafana/test
make test
# or: tilt ci
```

Unit tests run automatically during Tiltfile evaluation and exit with success or failure.

### Integration Tests

```bash
cd composables/grafana/test
make test-integration
# or: tilt ci -f Tiltfile.integration
```

Integration tests verify the compose_composer pipeline and staged files:
- Loads grafana with k3s-apiserver and mysql dependencies
- Verifies wire-when rules activate correctly
- Checks all expected values in staged grafana.yaml
- Fails if any expected configuration is missing

## Test Coverage

### provision_plugins() Helper

Tests for the plugin provisioning helper function:

- ✓ Empty list returns empty dict
- ✓ Single plugin path generates correct structure
- ✓ Multiple plugin paths create unique mount points
- ✓ Volume mount format is correct (path:mount:ro)
- ✓ Mount points are sequential (plugin-0, plugin-1, etc.)

**Coverage:** All positive test cases and edge cases

### register_aggregator_config() Helper

Tests for the API aggregation configuration helper. This helper uses the accumulation
pattern - it returns a marker dict that `process_accumulated_modifications()` collects
and merges into a single aggregator-config service.

**register_aggregator_config Tests:**
- ✓ Returns marker dict with `_aggregator_api_groups` key
- ✓ Preserves api_groups in marker for later processing

**process_accumulated_modifications Tests:**
- ✓ Single API group generates correct service
- ✓ Multiple API groups from different composables are accumulated
- ✓ Duplicate groups are deduplicated by group name
- ✓ Default values (host='k3s-apiserver', port=6443)
- ✓ Custom host and port values preserved
- ✓ Returns empty dict when no api_groups markers present

**Error Validation:** The helper includes fail() calls for:
- Empty api_groups list
- Non-list api_groups parameter
- Non-dict items in api_groups
- Missing required fields (group, version)

These error cases are validated at runtime when the helper is called with invalid input.

## Test Structure

The test suite follows the pattern established by `composables/k3s-apiserver/test/`:

1. **Function Definitions** - Helper functions copied for testing
2. **Test Helpers** - Assertion utilities (assert_equals, assert_true, assert_in, assert_contains)
3. **Test Functions** - Individual test cases for each scenario
4. **Test Runner** - Executes all tests and reports results

## Adding New Tests

To add a new test:

1. Define a test function with descriptive name:
   ```python
   def test_my_new_scenario():
       """Test description."""
       result = provision_plugins(...)
       assert_equals(result, expected)
       print("  [PASS] test_my_new_scenario")
   ```

2. Add the test to `run_tests()`:
   ```python
   def run_tests():
       # ... existing tests ...
       test_my_new_scenario()
   ```

3. Run tests to verify:
   ```bash
   tilt ci
   ```

## Integration Test Details

The integration test (`Tiltfile.integration`) automatically verifies:

**Wire-When Rules Activation:**
- ✓ grafana depends on db (mysql dependency)
- ✓ grafana depends on k3s-apiserver (k8s dependency)

**MySQL Wire-When Configuration:**
- ✓ GF_DATABASE_TYPE: mysql
- ✓ GF_DATABASE_HOST: db:3306
- ✓ GF_DATABASE_NAME: grafana
- ✓ GF_SESSION_PROVIDER: mysql

**K8s Wire-When Configuration:**
- ✓ KUBECONFIG: /etc/grafana/kubeconfig.yaml
- ✓ GF_GRAFANA_APISERVER_PROXY_CLIENT_CERT_FILE configured
- ✓ GF_GRAFANA_APISERVER_PROXY_CLIENT_KEY_FILE configured
- ✓ GF_GRAFANA_APISERVER_REMOTE_SERVICES_FILE: /etc/kubernetes/pki/aggregator-config.yaml
- ✓ k3s-certs volume mounted at /etc/kubernetes/pki:ro

**Helper Modifications:**
- ✓ aggregator-config service modified by register_aggregator_config()
- ✓ API group test.ext.grafana.com registered
- ✓ API version v1 registered
- ✓ API host k3s-apiserver configured
- ✓ API port 6443 configured

The test reads the staged `.cc/grafana.yaml` file and fails if any expected value is missing. This ensures wire-when rules and helper modifications work correctly.

### Runtime Verification

To verify runtime behavior with actual services:

```bash
cd service-model
export COMPOSABLES_URL='file:///path/to/composables'
tilt up

# Verify aggregator-config service
docker compose logs aggregator-config

# Verify config file
docker compose exec grafana cat /etc/kubernetes/pki/aggregator-config.yaml

# Verify Grafana loads config
docker compose logs grafana | grep -i "apiserver\|aggregat"
```

## Wire-When Integration Tests

Testing declarative wiring rules requires loading dependencies:

1. **mysql trigger**: Test database configuration
   ```bash
   # Load grafana with mysql dependency
   tilt up
   # Verify GF_DATABASE_TYPE, GF_DATABASE_HOST, etc.
   ```

2. **k3s-apiserver trigger**: Test k8s integration
   ```bash
   # Load grafana with k3s-apiserver
   tilt up
   # Verify KUBECONFIG, cert mounts, aggregator env vars
   ```

3. **nats trigger**: Test NATS integration
   ```bash
   # Load grafana with nats
   tilt up
   # Verify DASH_PKG_NATS_SERVER_ADDRESS
   ```

See `composables/grafana/Tiltfile` for complete wire-when rules.

## Test Results

Current test status: **All tests passing** ✓

```
=== provision_plugins Tests ===
  [PASS] test_provision_plugins_empty_list
  [PASS] test_provision_plugins_single_path
  [PASS] test_provision_plugins_multiple_paths
  [PASS] test_provision_plugins_volume_format
  [PASS] test_provision_plugins_unique_mount_points

=== register_aggregator_config Tests ===
  [PASS] test_register_aggregator_config_returns_marker
  [PASS] test_register_aggregator_config_preserves_api_groups

=== process_accumulated_modifications Tests ===
  [PASS] test_process_accumulated_modifications_single
  [PASS] test_process_accumulated_modifications_multiple
  [PASS] test_process_accumulated_modifications_deduplicates
  [PASS] test_process_accumulated_modifications_default_host_port
  [PASS] test_process_accumulated_modifications_custom_host_port
  [PASS] test_process_accumulated_modifications_empty

==================================================
All tests passed!
==================================================
```

## Future Test Enhancements

Potential areas for expanded test coverage:

1. **cc_wire_when() validation** - Verify wire-when rules produce correct compose overrides
2. **Runtime behavior tests** - Automated tests that start containers and verify runtime behavior
3. **Performance tests** - Test behavior with large numbers of API groups or plugin paths
4. **Error recovery tests** - Test graceful handling of malformed YAML or missing files

## Related Documentation

- `composables/grafana/Tiltfile` - Helper function implementations
- `composables/grafana/grafana.yaml` - Service definitions
- `composables/k3s-apiserver/test/` - Reference test implementation
- `service-model/` - Integration test examples
