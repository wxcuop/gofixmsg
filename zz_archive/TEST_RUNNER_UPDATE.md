# Test Runner Updates

## New Comprehensive Test Option

The test runner has been updated to include a new `--comprehensive` option for running the Phase 2 comprehensive test suites.

### Usage

```bash
# Run comprehensive test suites (Phase 2)
python test_runner.py --comprehensive

# Run with verbose output
python test_runner.py --comprehensive --verbose

# Run all available options
python test_runner.py --help
```

### Comprehensive Test Files Included

The `--comprehensive` option runs these specific test files:

1. `tests/unit/test_engine_comprehensive.py` - Complete engine session management tests
2. `tests/unit/test_state_machine_comprehensive.py` - Complete state transition tests  
3. `tests/unit/test_heartbeat_comprehensive.py` - Complete heartbeat lifecycle tests
4. `tests/unit/test_network_comprehensive.py` - Complete network connection tests

### Coverage Results

When run, the comprehensive tests provide:
- **48 comprehensive tests** covering core FixEngine functionality
- **35% overall FixEngine coverage** 
- **Module-specific coverage:**
  - Network: 71% coverage
  - State Machine: 82% coverage
  - Heartbeat: 42% coverage
  - Engine: 25% coverage

### Integration with Existing Test Categories

The comprehensive tests are also included in the regular `--unit` option, so they will run as part of normal unit test execution. The `--comprehensive` option provides a focused way to run just these specific high-value test suites.

### Example Output

```bash
$ python test_runner.py --comprehensive

================================================================================
RUNNING COMPREHENSIVE TEST SUITES (PHASE 2)
================================================================================
Running command: python -m pytest -v --tb=short --cov=pyfixmsg_plus.fixengine --cov-report=term-missing tests/unit/test_engine_comprehensive.py tests/unit/test_state_machine_comprehensive.py tests/unit/test_heartbeat_comprehensive.py tests/unit/test_network_comprehensive.py

=============================== 48 passed in 1.20s ================================

COMPREHENSIVE TESTS COMPLETED
Status: PASSED
Duration: 2.02 seconds
```
