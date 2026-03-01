# PyFixMsg Plus: Lessons Learnt (Phase 2)

## Context
- **Goal:** Achieve 95% test coverage for production readiness (Phase 2)
- **Focus:** Session management, message store flexibility, async/await support, QuickFIX/J interoperability, and robust testing
- **Key Files:** `test_engine_core.py`, `conftest.py`, `test_runner.py`, `test_engine_mocked.py`

---

## 1. Async Scheduler Issue & Mocking Strategy
- **Problem:** `Scheduler` in `engine.py` uses `asyncio.create_task()` in constructor, causing `RuntimeError: no running event loop` in tests.
- **Lesson:** Direct async initialization in constructors breaks testability and coverage.
- **Solution:** Mock the scheduler completely in tests using `unittest.mock.Mock` and `AsyncMock`.
- **Result:** Unblocked all engine tests, enabled rapid coverage gains, and avoided production code changes.

---

## 2. Mocking vs. Lazy Initialization
- **Option 1 (Mock Scheduler):**
  - ✅ Immediate coverage wins
  - ✅ No production code changes
  - ✅ Fast, predictable test execution
  - ❌ Scheduler integration not tested in unit tests
- **Option 2 (Lazy Initialization):**
  - ✅ Real integration coverage
  - ❌ Requires refactoring production code
  - ❌ More complex async test setup
- **Decision:** Use Option 1 for Phase 2, defer Option 2 to Phase 3 (integration tests)

---

## 3. Test Infrastructure & Coverage Expansion
- **Pytest Fixtures:** Centralized scheduler mock in `conftest.py` for reuse
- **Patch Decorator:** Use `@patch('pyfixmsg_plus.fixengine.engine.Scheduler')` to inject mock in all engine tests
- **Coverage Impact:**
  - Engine coverage: 0% → 18%
  - Overall project coverage: 22% → 27%
  - All unit tests passing (23/23)
- **Lesson:** Systematic mocking and patching unlocks coverage for async-dependent components

---

## 4. Duplicate Class Definitions
- **Problem:** Duplicate `TestFixEngineCore` class caused test confusion and missing methods
- **Lesson:** Always check for duplicate test classes; pytest may run the wrong one
- **Solution:** Remove duplicates, consolidate helpers and fixtures

---

## 5. Coverage Strategy & Next Steps
- **High-Impact Targets:** Focus on engine, message handler, and message store for maximum coverage
- **Test Runner Integration:** Add working tests to `test_runner.py` for CI/CD
- **Lesson:** Prioritize high-impact files and working tests for rapid progress
- **Next:** Expand mocking pattern to message handler and database message store

---

## 6. General Best Practices
- **Isolate Async Complexity:** Use mocks for async components in unit tests
- **Patch Imports, Not Instances:** Patch at the import site for reliable mocking
- **Clean Up Temp Files:** Always remove temp files after tests
- **Test Property & Method Access:** Cover both property getters/setters and method existence
- **Document Decisions:** Save lessons learnt for future contributors

---

## 7. Summary Table
| Area                | Problem/Decision                | Solution/Outcome                |
|---------------------|---------------------------------|---------------------------------|
| Scheduler Async     | No event loop in tests          | Mock scheduler, patch import    |
| Coverage Strategy   | Blocked by async                | Systematic mocking, patching    |
| Duplicate Classes   | Test confusion                  | Remove/merge duplicates         |
| Test Runner         | CI/CD integration               | Add only working tests          |
| Next Steps          | Expand coverage                 | Target message handler/store    |

---

## 8. Final Recommendation
- **Mock async dependencies for rapid coverage in Phase 2**
- **Defer integration testing and lazy initialization refactor to Phase 3**
- **Document all lessons and decisions for future maintainers**

---

*Saved: July 26, 2025*
