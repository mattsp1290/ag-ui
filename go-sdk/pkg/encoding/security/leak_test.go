package security

import (
	"os"
	"testing"
	
	"github.com/ag-ui/go-sdk/pkg/encoding"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	// Run tests
	code := m.Run()
	
	// Cleanup global registry to stop background goroutine
	encoding.CloseGlobalRegistry()
	
	// Exit with test result
	os.Exit(code)
}

// Add this to enable goleak checking per test instead of at package level
func TestNoGoroutineLeaks(t *testing.T) {
	// Close any existing global registry to stop background goroutines
	encoding.CloseGlobalRegistry()
	
	defer goleak.VerifyNone(t,
		goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"),
		goleak.IgnoreTopFunction("github.com/golang/glog.(*loggingT).flushDaemon"),
		goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"),
	)
}