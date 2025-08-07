# Test Interference Debugging Scripts

This directory contains three specialized scripts to help isolate and debug the goroutine leak issue in `TestGoroutineLifecycleManager/GracefulShutdown`.

## Problem Description

The test `TestGoroutineLifecycleManager/GracefulShutdown` fails when run with the full test suite but passes when run in isolation. The error indicates a "metrics" goroutine is not being cleaned up properly, causing the test to report "Expected 0 active goroutines after shutdown, got 1".

## Analysis Summary

Based on code analysis, the issue is caused by test interference between:
1. `TestGoroutineLifecycleIntegration/RealisticUsage` (line ~407) - creates "metrics" ticker
2. `TestComprehensiveGoroutineLifecycle/RealWorldScenario` (line ~482) - creates "metrics" ticker via TestService

## Scripts Overview

### 1. `quick-interference-test.sh` ⚡
**Best for: Immediate diagnosis**

Quick 30-second test to confirm the interference and identify the culprit.

```bash
./quick-interference-test.sh
```

**What it does:**
- Runs failing test in isolation
- Tests specific combinations with suspect tests
- Provides immediate diagnosis
- Saves detailed output in `/tmp/test[1-4].out`

### 2. `test-interference-isolator.sh` 🔬
**Best for: Comprehensive analysis**

Full systematic analysis of the test interference issue.

```bash
./test-interference-isolator.sh

# Options:
./test-interference-isolator.sh --quick    # Skip some phases
./test-interference-isolator.sh --race     # Focus on race detection
./test-interference-isolator.sh --verbose  # Enable verbose logging
```

**What it does:**
- 7-phase comprehensive testing
- Tests different concurrency settings (-p=1, -p=2, -p=4)
- Race detection analysis
- Sequential vs parallel execution testing
- Generates detailed analysis report
- Results saved in `./test-interference-results/`

### 3. `goroutine-leak-debugger.sh` 🐛
**Best for: Deep goroutine analysis**

Detailed goroutine-level debugging with custom tracking.

```bash
./goroutine-leak-debugger.sh
```

**What it does:**
- Tracks goroutine creation/cleanup in detail
- Categorizes goroutines by type (metrics, ticker, worker, etc.)
- Provides goroutine stack traces
- Tests specific interference scenarios
- Results saved in `./goroutine-debug-results/`

## Usage Recommendations

### Quick Diagnosis (2 minutes)
```bash
./quick-interference-test.sh
```

### Full Analysis (5-10 minutes)
```bash
./test-interference-isolator.sh
```

### Deep Debugging (3-5 minutes)
```bash
./goroutine-leak-debugger.sh
```

## Expected Results

If the hypothesis is correct, you should see:

1. **quick-interference-test.sh**: Shows that the failing test passes alone but fails with specific combinations
2. **test-interference-isolator.sh**: Identifies specific test combinations that cause failures
3. **goroutine-leak-debugger.sh**: Shows "metrics" goroutines not being cleaned up properly

## Root Cause

The issue is caused by:

1. **Shared Naming**: Multiple tests create goroutines named "metrics"
2. **Improper Cleanup**: TestService in RealWorldScenario may not properly shut down
3. **Test Order Dependency**: Tests are not properly isolated

## Recommended Fixes

### Immediate Fixes

1. **Unique Goroutine Names**: 
   ```go
   // Instead of:
   manager.GoTicker("metrics", ...)
   
   // Use:
   manager.GoTicker(fmt.Sprintf("metrics-%s", t.Name()), ...)
   ```

2. **Verify TestService Cleanup**:
   ```go
   // In TestService.Stop(), ensure proper cleanup:
   func (s *TestService) Stop() error {
       s.mu.Lock()
       defer s.mu.Unlock()
       
       if !s.started {
           return nil
       }
       
       close(s.inputCh)
       s.started = false
       
       // Add explicit timeout and verification
       ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
       defer cancel()
       
       if err := s.manager.ShutdownWithContext(ctx); err != nil {
           return fmt.Errorf("failed to shutdown manager: %w", err)
       }
       
       // Verify no goroutines remain
       if s.manager.GetActiveCount() != 0 {
           return fmt.Errorf("goroutines still active after shutdown: %d", s.manager.GetActiveCount())
       }
       
       return nil
   }
   ```

3. **Add Explicit Cleanup Verification**:
   ```go
   // In test cleanup:
   defer func() {
       if manager.GetActiveCount() != 0 {
           t.Errorf("Test cleanup failed: %d goroutines still active", manager.GetActiveCount())
           // Print active goroutines for debugging
           for id, info := range manager.GetActiveGoroutines() {
               t.Logf("Active goroutine: %s (running: %v)", id, info.Running)
           }
       }
   }()
   ```

### Files to Modify

1. `/Users/punk1290/git/ag-ui/go-sdk/pkg/testing/goroutine_lifecycle_test.go`
   - Line ~407: Change "metrics" to test-specific name
   
2. `/Users/punk1290/git/ag-ui/go-sdk/pkg/testing/comprehensive_goroutine_test.go`
   - Line ~482: Change "metrics" to test-specific name
   - Review TestService.Stop() method for proper cleanup

## Troubleshooting

### Scripts Don't Run
- Ensure you're in the go-sdk root directory
- Check that Go is installed and in PATH
- Make scripts executable: `chmod +x *.sh`

### No Issues Detected
- The issue might be intermittent - run scripts multiple times
- Try with different concurrency settings
- Check if recent changes fixed the issue

### Scripts Fail
- Check Go module dependencies: `go mod tidy`
- Ensure test files haven't been moved or renamed
- Verify network access for any dependencies

## Output Files

### quick-interference-test.sh
- `/tmp/test[1-4].out` - Individual test outputs

### test-interference-isolator.sh
- `./test-interference-results/` - Comprehensive results
- `./test-interference-results/INTERFERENCE_ANALYSIS_REPORT.md` - Summary report

### goroutine-leak-debugger.sh
- `./goroutine-debug-results/` - Detailed goroutine analysis
- `./goroutine-debug-results/GOROUTINE_LEAK_ANALYSIS.md` - Analysis report

## Next Steps

1. Run the quick test to confirm the issue
2. Apply the recommended fixes
3. Re-run tests to verify the fix
4. Consider adding automated goroutine leak detection to CI

## Support

If the scripts don't identify the issue or you need help interpreting results, check the detailed output files and look for:
- Goroutine stack traces containing "metrics"
- Tests that pass individually but fail in combination
- Cleanup timeouts or shutdown errors

The scripts are designed to be self-documenting with detailed logs and analysis reports.