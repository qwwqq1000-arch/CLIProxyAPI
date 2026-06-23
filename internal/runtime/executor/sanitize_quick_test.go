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

func TestSanitizeQuick_EmptyText_Removed(t *testing.T) {
	in := []byte(`{"messages":[{"role":"user","content":[{"type":"text","text":""},{"type":"text","text":"hi"}]}],"system":[{"type":"text","text":"   "},{"type":"text","text":"keep"}]}`)
	out := sanitizeClaudeUpstreamRequest(in)
	// empty block dropped → "hi" is now index 0, and only 1 block remains
	if n := gjson.GetBytes(out, "messages.0.content.#").Int(); n != 1 {
		t.Fatalf("expected 1 content block after removal, got %d; body=%s", n, out)
	}
	if got := gjson.GetBytes(out, "messages.0.content.0.text").String(); got != "hi" {
		t.Fatalf("kept block wrong, got %q; body=%s", got, out)
	}
	// whitespace system block dropped, "keep" remains
	if n := gjson.GetBytes(out, "system.#").Int(); n != 1 {
		t.Fatalf("expected 1 system block, got %d; body=%s", n, out)
	}
	if got := gjson.GetBytes(out, "system.0.text").String(); got != "keep" {
		t.Fatalf("system kept wrong, got %q", got)
	}
}

func TestSanitizeQuick_AllEmpty_Placeholder(t *testing.T) {
	in := []byte(`{"messages":[{"role":"user","content":[{"type":"text","text":""}]}],"system":[{"type":"text","text":""}]}`)
	out := sanitizeClaudeUpstreamRequest(in)
	if got := gjson.GetBytes(out, "messages.0.content.0.text").String(); got != "." {
		t.Fatalf("expected placeholder '.', got %q; body=%s", got, out)
	}
	if gjson.GetBytes(out, "system").Exists() {
		t.Fatalf("emptied system should be dropped; body=%s", out)
	}
}

func TestSanitizeQuick_NoopOnWellFormed(t *testing.T) {
	in := []byte(`{"tools":[{"type":"web_search_20250305","name":"web_search"}],"messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`)
	out := sanitizeClaudeUpstreamRequest(in)
	if string(out) != string(in) {
		t.Fatalf("modified well-formed body:\n in=%s\nout=%s", in, out)
	}
}
