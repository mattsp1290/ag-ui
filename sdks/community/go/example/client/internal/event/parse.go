package event

import (
	"fmt"

	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

func Parse(data []byte) (events.Event, error) {
	event, err := events.EventFromJSON(data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode SSE event: %w", err)
	}

	return event, nil
}
