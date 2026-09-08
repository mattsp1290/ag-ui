package agent

import (
	"context"
	"time"

	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
	aguitypes "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/types"
)

// ReasoningDemo is a scripted protocol demonstration. Its reasoning text is
// explanatory UI content and does not represent model internals.
type ReasoningDemo struct{ Pace time.Duration }

func (d ReasoningDemo) Run(ctx context.Context, emit *Emitter, in *aguitypes.RunAgentInput, threadID, runID string) {
	emit.RunStarted()
	reasoningID := events.GenerateMessageID()
	answerID := events.GenerateMessageID()
	reasoningChunks := []string{"Reviewing the request. ", "Organizing a clear response."}
	answerChunks := []string{"The scripted reasoning demonstration is complete."}

	emit.ReasoningStart(reasoningID)
	emit.ReasoningMessageStart(reasoningID)
	for _, chunk := range reasoningChunks {
		if !d.emitChunk(ctx, emit, func() { emit.ReasoningContent(reasoningID, chunk) }) {
			return
		}
	}
	emit.ReasoningMessageEnd(reasoningID)
	emit.ReasoningEnd(reasoningID)
	if ctx.Err() != nil || emit.Err() != nil {
		return
	}

	emit.TextStart(answerID)
	for _, chunk := range answerChunks {
		if !d.emitChunk(ctx, emit, func() { emit.TextContent(answerID, chunk) }) {
			return
		}
	}
	emit.TextEnd(answerID)
	if ctx.Err() != nil || emit.Err() != nil {
		return
	}
	emit.MessagesSnapshot([]aguitypes.Message{
		{ID: reasoningID, Role: aguitypes.RoleReasoning, Content: reasoningChunks[0] + reasoningChunks[1]},
		{ID: answerID, Role: aguitypes.RoleAssistant, Content: answerChunks[0]},
	})
	if ctx.Err() != nil || emit.Err() != nil {
		return
	}
	emit.RunFinishedSuccess()
}

func (d ReasoningDemo) emitChunk(ctx context.Context, emit *Emitter, write func()) bool {
	if ctx.Err() != nil || emit.Err() != nil {
		return false
	}
	write()
	if d.Pace <= 0 {
		return ctx.Err() == nil && emit.Err() == nil
	}
	timer := time.NewTimer(d.Pace)
	defer timer.Stop()
	select {
	case <-timer.C:
		return emit.Err() == nil
	case <-ctx.Done():
		return false
	}
}
