package sse

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

type Event struct {
	Type        string            `json:"type"`
	AgentName   string            `json:"agent_name,omitempty"`
	RunPath     string            `json:"run_path,omitempty"`
	Content     string            `json:"content,omitempty"`
	ToolCalls   []schema.ToolCall `json:"tool_calls,omitempty"`
	ActionType  string            `json:"action_type,omitempty"`
	InterruptID string            `json:"interrupt_id,omitempty"`
	Error       string            `json:"error,omitempty"`
}

type Writer struct{}

func NewWriter() *Writer {
	return &Writer{}
}

func (w *Writer) WriteEvent(writer io.Writer, event Event) error {
	return writeEvent(writer, event)
}

func (w *Writer) Write(writer io.Writer, event *adk.AgentEvent) error {
	if event.Err != nil {
		return writeEvent(writer, Event{
			Type:      "error",
			AgentName: event.AgentName,
			RunPath:   formatRunPath(event.RunPath),
			Error:     event.Err.Error(),
		})
	}

	if event.Output != nil && event.Output.MessageOutput != nil {
		if err := w.writeMessageOutput(writer, event); err != nil {
			return err
		}
	}

	if event.Action != nil {
		return w.writeAction(writer, event)
	}
	return nil
}

func (w *Writer) writeMessageOutput(writer io.Writer, event *adk.AgentEvent) error {
	msgOutput := event.Output.MessageOutput
	if msg := msgOutput.Message; msg != nil {
		if msg.Role == schema.Tool {
			return nil
		}
		return writeEvent(writer, Event{
			Type:      "message",
			AgentName: event.AgentName,
			RunPath:   formatRunPath(event.RunPath),
			Content:   msg.Content,
		})
	}

	if stream := msgOutput.MessageStream; stream != nil {
		return w.writeStream(writer, event, stream)
	}
	return nil
}

func (w *Writer) writeStream(writer io.Writer, event *adk.AgentEvent, stream *schema.StreamReader[*schema.Message]) error {
	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return writeEvent(writer, Event{
				Type:      "error",
				AgentName: event.AgentName,
				RunPath:   formatRunPath(event.RunPath),
				Error:     fmt.Sprintf("stream error: %v", err),
			})
		}
		if chunk.Content != "" && chunk.Role != schema.Tool {
			if err := writeEvent(writer, Event{
				Type:      "stream_chunk",
				AgentName: event.AgentName,
				RunPath:   formatRunPath(event.RunPath),
				Content:   chunk.Content,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (w *Writer) writeAction(writer io.Writer, event *adk.AgentEvent) error {
	action := event.Action
	if action.TransferToAgent != nil {
		return writeEvent(writer, Event{
			Type:       "action",
			AgentName:  event.AgentName,
			RunPath:    formatRunPath(event.RunPath),
			ActionType: "transfer",
			Content:    action.TransferToAgent.DestAgentName,
		})
	}
	if action.Interrupted != nil {
		for _, ic := range action.Interrupted.InterruptContexts {
			content := fmt.Sprintf("%v", ic.Info)
			if stringer, ok := ic.Info.(fmt.Stringer); ok {
				content = stringer.String()
			}
			if err := writeEvent(writer, Event{
				Type:        "action",
				AgentName:   event.AgentName,
				RunPath:     formatRunPath(event.RunPath),
				ActionType:  "interrupted",
				InterruptID: ic.ID,
				Content:     content,
			}); err != nil {
				return err
			}
		}
	}
	if action.Exit {
		return writeEvent(writer, Event{
			Type:       "action",
			AgentName:  event.AgentName,
			RunPath:    formatRunPath(event.RunPath),
			ActionType: "exit",
			Content:    "agent execution completed",
		})
	}
	return nil
}

func writeEvent(writer io.Writer, event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(writer, "data: %s\n\n", data)
	return err
}

func formatRunPath(runPath []adk.RunStep) string {
	return fmt.Sprintf("%v", runPath)
}
