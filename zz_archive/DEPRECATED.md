# DEPRECATED: Python FIX Engine Runtime

As of March 1, 2026, the Python implementation of the FIX engine (`pyfixmsg_plus`) is deprecated in favor of the new Go-based implementation (`gofixmsg`).

## Why the Switch to Go?

The Go implementation provides significant improvements:

| Aspect | Python | Go |
|--------|--------|-----|
| **Latency** | ~100-200ms per round-trip | ~10-20ms per round-trip |
| **Throughput** | ~1000 msgs/sec per session | ~10,000-50,000 msgs/sec per session |
| **Concurrency** | Single-threaded (asyncio) | Multi-core goroutines |
| **Memory** | Higher memory footprint | Significantly lower |
| **Type Safety** | Runtime errors possible | Compile-time checking |
| **Security** | Limited TLS validation | Full CA chain verification |

## Migration Timeline

| Phase | Date | Action |
|-------|------|--------|
| **Deprecated** | March 1, 2026 | Mark Python runtime as deprecated (this date) |
| **Current Release** | Now | Both Python and Go available; Go recommended |
| **Next Major Release** | TBD | Python FIX engine removed from core |
| **Sunset** | TBD | Python code moved to archive-only repository |

## Migration Path

### For Existing Python Users

1. **Review Migration Guide**: Read [MIGRATION.md](../gofixmsg/doc/migration.md)
2. **Test in Staging**: Run Go version alongside Python in a test environment
3. **Validate Behavior**: Compare logs, sequence numbers, and message handling
4. **Plan Cutover**: Schedule production migration during low-volume period
5. **Execute Migration**: Switch to Go and monitor for issues
6. **Rollback Plan**: Have Python service ready to restart if needed

### Quick Start with Go

```bash
cd gofixmsg

# Build examples
go build -o acceptor examples/acceptor/main.go
go build -o initiator examples/initiator/main.go

# Run tests
go test ./... -v
```

See [Examples README](../gofixmsg/examples/README.md) for detailed instructions.

## Python Code Location

The deprecated Python code is archived in [zz_archive/](../zz_archive/). It includes:

- `pyfixmsg/` - Core FIX message library
- `pyfixmsg_plus/` - FIX session management (asyncio-based)
- `tests/` - Complete test suite
- `examples/` - Example applications
- Configuration and documentation

## What's Changing?

### Remove from Requirements

```bash
# Python setup is NO LONGER RECOMMENDED
# Do not run: pip install -r requirements.txt

# Instead, use Go:
cd gofixmsg && go mod download && go test ./...
```

### Update Imports

**Python (deprecated):**
```python
from pyfixmsg_plus.fixengine import FixEngine
from pyfixmsg_plus.application import Application
```

**Go (new):**
```go
import (
    "github.com/wxcuop/gofixmsg/engine"
    "github.com/wxcuop/gofixmsg/fixmsg"
)
```

See [API Mapping](../gofixmsg/doc/migration.md#api-examples) for detailed comparison.

## Sunset Gate Verification

Before deprecating, we verified:

- ✅ All Go tests passing (`go test ./...`)
- ✅ Integration suite green (multi-session, TLS, reconnection)
- ✅ Migration documentation complete
- ✅ Examples published and tested
- ✅ Risk assessment completed
- ✅ Rollback procedures documented

## Risk Assessment

### Why Migrate to Go?

**No functional regression expected.** The Go version:
1. Uses the same `.ini` configuration format
2. Supports the same SQLite schema for message stores
3. Implements identical state machine logic
4. Provides callback API equivalent to Python

### Migration Risks

| Risk | Mitigation |
|------|-----------|
| Behavior differences in timing | Run both in parallel for validation |
| Configuration format errors | Use provided template and validate |
| Database incompatibility | Both use same SQLite schema |
| Unknown edge cases | Run canary in production with fallback |

## Support and Questions

### For Python Users

- **Support**: Refer to archived documentation in `zz_archive/`

### For Go Users

- **New Implementations**: use `gofixmsg` (this module)
- **Documentation**: See [gofixmsg/doc/](../gofixmsg/doc/)
- **Examples**: Ready-to-run examples in [gofixmsg/examples/](../gofixmsg/examples/)

## References

- [Python to Go Migration Guide](../gofixmsg/doc/migration.md)
- [Sunset Recommendation](../gofixmsg/doc/sunset_recommendation.md)
- [GoFixMsg Examples](../gofixmsg/examples/)
- [GoFixMsg README](../README.md)


