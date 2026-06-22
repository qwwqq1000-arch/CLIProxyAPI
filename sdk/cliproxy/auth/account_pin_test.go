package auth

import (
	"net/http"
	"net/url"
	"testing"

	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/executor"
)

func TestMatchPinnedAuth(t *testing.T) {
	auths := []*Auth{
		{ID: "id-1", FileName: "/root/.cli-proxy-api/claude-a@gmail.com.json", Label: "A", Attributes: map[string]string{"email": "claude-a@gmail.com"}},
		{ID: "id-2", FileName: "claude-b@gmail.com.json", Attributes: map[string]string{"email": "claude-b@gmail.com"}},
	}
	for _, a := range auths {
		a.EnsureIndex()
	}
	cases := map[string]string{
		"id-2":                  "id-2", // by id
		"claude-a@gmail.com":    "id-1", // by email
		"claude-a@gmail.com.json": "id-1", // by filename
		"A":                     "id-1", // by label
		"CLAUDE-B@GMAIL.COM":    "id-2", // case-insensitive email
	}
	for pin, wantID := range cases {
		got := matchPinnedAuth(auths, pin)
		if got == nil || got.ID != wantID {
			t.Errorf("pin %q → %v, want %s", pin, got, wantID)
		}
	}
	if matchPinnedAuth(auths, "nope") != nil {
		t.Error("unknown pin should not match")
	}
}

func TestAccountPin_HeaderAndQuery(t *testing.T) {
	h := http.Header{}
	h.Set(AccountPinHeader, "id-1")
	if got := accountPin(cliproxyexecutor.Options{Headers: h}); got != "id-1" {
		t.Errorf("header pin = %q", got)
	}
	q := url.Values{}
	q.Set("account", "id-2")
	if got := accountPin(cliproxyexecutor.Options{Query: q}); got != "id-2" {
		t.Errorf("query pin = %q", got)
	}
	if got := accountPin(cliproxyexecutor.Options{}); got != "" {
		t.Errorf("no pin should be empty, got %q", got)
	}
}

func TestRoundRobin_HonorsPin(t *testing.T) {
	auths := []*Auth{
		{ID: "id-1", Provider: "claude", Status: StatusActive},
		{ID: "id-2", Provider: "claude", Status: StatusActive},
		{ID: "id-3", Provider: "claude", Status: StatusActive},
	}
	for _, a := range auths {
		a.EnsureIndex()
	}
	h := http.Header{}
	h.Set(AccountPinHeader, "id-3")
	s := &RoundRobinSelector{}
	for i := 0; i < 5; i++ {
		got, err := s.Pick(nil, "claude", "claude-x", cliproxyexecutor.Options{Headers: h}, auths)
		if err != nil || got == nil || got.ID != "id-3" {
			t.Fatalf("pinned pick = %v err=%v, want id-3 every time", got, err)
		}
	}
	// Unknown pin → error, not silent fallback.
	h2 := http.Header{}
	h2.Set(AccountPinHeader, "ghost")
	if _, err := s.Pick(nil, "claude", "claude-x", cliproxyexecutor.Options{Headers: h2}, auths); err == nil {
		t.Fatal("unknown pin should error")
	}
}
