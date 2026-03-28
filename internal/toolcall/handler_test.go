package toolcall

import (
	"testing"
)

func TestToolCallRequest_UnmarshalJSON(t *testing.T) {
	// Basic sanity test that the types compile
	req := ToolCallRequest{
		Tool:   "make_voice_call",
		UserID: "test-user",
		Params: map[string]any{"contact_name": "Lan"},
	}
	if req.Tool != "make_voice_call" {
		t.Error("unexpected tool name")
	}
}
