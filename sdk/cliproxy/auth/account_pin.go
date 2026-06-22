package auth

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/executor"
)

// AccountPinHeader is the request header that pins a single inference request to
// a specific account/credential, bypassing the normal load-balancing selector.
// The value is matched (case-insensitively) against an auth's id, runtime index,
// label, backing file name (with or without the .json suffix), or email
// attribute. The pinned account must still be available (not disabled, not in
// cooldown); otherwise the request fails rather than silently falling back.
//
// A query parameter ("account" or "auth_index") is also accepted, which is handy
// for clients that cannot set custom headers.
const AccountPinHeader = "X-CLIProxy-Account"

// accountPin returns the pin requested by the caller, or "" if none.
func accountPin(opts cliproxyexecutor.Options) string {
	if v := pinFromHeader(opts.Headers); v != "" {
		return v
	}
	return pinFromQuery(opts.Query)
}

func pinFromHeader(h http.Header) string {
	if h == nil {
		return ""
	}
	if v := strings.TrimSpace(h.Get(AccountPinHeader)); v != "" {
		return v
	}
	// Accept a couple of friendly aliases.
	if v := strings.TrimSpace(h.Get("X-CLIProxy-Auth-Index")); v != "" {
		return v
	}
	return ""
}

func pinFromQuery(q url.Values) string {
	if q == nil {
		return ""
	}
	if v := strings.TrimSpace(q.Get("account")); v != "" {
		return v
	}
	return strings.TrimSpace(q.Get("auth_index"))
}

// authMatchesPin reports whether a single auth matches the pin value, by id,
// runtime index, label, backing file name (with/without .json), or email.
func authMatchesPin(a *Auth, pin string) bool {
	if a == nil || pin == "" {
		return false
	}
	a.EnsureIndex()
	if strings.EqualFold(a.ID, pin) ||
		strings.EqualFold(strings.TrimSuffix(a.ID, ".json"), pin) ||
		strings.EqualFold(a.Index, pin) ||
		(a.Label != "" && strings.EqualFold(a.Label, pin)) {
		return true
	}
	if a.FileName != "" {
		base := a.FileName
		if i := strings.LastIndexAny(base, "/\\"); i >= 0 {
			base = base[i+1:]
		}
		if strings.EqualFold(base, pin) || strings.EqualFold(strings.TrimSuffix(base, ".json"), pin) {
			return true
		}
	}
	if a.Attributes != nil {
		if email := strings.TrimSpace(a.Attributes["email"]); email != "" && strings.EqualFold(email, pin) {
			return true
		}
	}
	return false
}

// matchPinnedAuth returns the auth from available that matches pin, or nil.
func matchPinnedAuth(available []*Auth, pin string) *Auth {
	if pin == "" {
		return nil
	}
	for _, a := range available {
		if authMatchesPin(a, pin) {
			return a
		}
	}
	return nil
}

// resolvePin resolves an account pin against ALL unblocked candidates across every
// priority tier (not just the top tier), so a lower-priority pinned account is
// still reachable. Returns (auth,true,nil) on match, (nil,false,nil) when no pin
// was requested, and (nil,true,err) when a pin matched nothing available.
func resolvePin(opts cliproxyexecutor.Options, auths []*Auth, model string, now time.Time) (*Auth, bool, error) {
	pin := accountPin(opts)
	if pin == "" {
		return nil, false, nil
	}
	byPriority, _, _ := collectAvailableByPriority(auths, model, now)
	for _, tier := range byPriority {
		if a := matchPinnedAuth(tier, pin); a != nil {
			return a, true, nil
		}
	}
	return nil, true, &Error{Code: "account_not_available", Message: "pinned account '" + pin + "' is not available (unknown, disabled, or cooling down)"}
}

// pinnedSelection resolves an account pin against the available candidates.
// Returns (auth, true, nil) when a pin matched, (nil, false, nil) when no pin
// was requested, and (nil, true, err) when a pin was requested but no available
// candidate matched (the caller should surface the error rather than fall back).
func pinnedSelection(opts cliproxyexecutor.Options, available []*Auth) (*Auth, bool, error) {
	pin := accountPin(opts)
	if pin == "" {
		return nil, false, nil
	}
	if a := matchPinnedAuth(available, pin); a != nil {
		return a, true, nil
	}
	return nil, true, &Error{Code: "account_not_available", Message: "pinned account '" + pin + "' is not available (unknown, disabled, or cooling down)"}
}
