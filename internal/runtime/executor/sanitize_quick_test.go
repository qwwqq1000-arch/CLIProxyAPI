package executor

import (
	"testing"

	"github.com/tidwall/gjson"
)

func TestSanitizeQuick_WebSearch(t *testing.T) {
	in := []byte(`{"model":"claude-haiku-4-5","tools":[{"input_schema":null,"name":"web_search"}],"tool_choice":{"name":"web_search","type":"tool"}}`)
	out := sanitizeClaudeUpstreamRequest(in)
	if got := gjson.GetBytes(out, "tools.0.type").String(); got != "web_search_20250305" {
		t.Fatalf("web_search type not set, got %q; body=%s", got, out)
	}
	if gjson.GetBytes(out, "tools.0.input_schema").Exists() {
		t.Fatalf("input_schema not removed; body=%s", out)
	}
}

func TestSanitizeQuick_EmptyText(t *testing.T) {
	in := []byte(`{"messages":[{"role":"user","content":[{"type":"text","text":""},{"type":"text","text":"hi"}]}],"system":[{"type":"text","text":"   "}]}`)
	out := sanitizeClaudeUpstreamRequest(in)
	if got := gjson.GetBytes(out, "messages.0.content.0.text").String(); got != " " {
		t.Fatalf("empty msg text not fixed, got %q; body=%s", got, out)
	}
	if got := gjson.GetBytes(out, "messages.0.content.1.text").String(); got != "hi" {
		t.Fatalf("non-empty text changed, got %q", got)
	}
	if got := gjson.GetBytes(out, "system.0.text").String(); got != " " {
		t.Fatalf("empty system text not fixed, got %q; body=%s", got, out)
	}
}

func TestSanitizeQuick_NoopOnWellFormed(t *testing.T) {
	in := []byte(`{"tools":[{"type":"web_search_20250305","name":"web_search"}],"messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`)
	out := sanitizeClaudeUpstreamRequest(in)
	if string(out) != string(in) {
		t.Fatalf("modified well-formed body:\n in=%s\nout=%s", in, out)
	}
}
