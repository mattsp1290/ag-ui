package sse

import (
	"testing"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m,
		goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"),
		goleak.IgnoreTopFunction("github.com/golang/glog.(*loggingT).flushDaemon"),
		goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"),
		goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"),
		goleak.IgnoreTopFunction("github.com/chromedp/chromedp.(*Browser).run"),
		goleak.IgnoreTopFunction("github.com/chromedp/chromedp.(*Target).run"),
		goleak.IgnoreTopFunction("github.com/chromedp/chromedp.NewContext.func1"),
	)
}