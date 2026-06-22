package management

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/runtime/executor/helps"
	coreauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
)

// anthropicUsageURL is the (undocumented) Anthropic OAuth usage endpoint that
// returns a Claude subscription account's rolling rate-limit windows.
const anthropicUsageURL = "https://api.anthropic.com/api/oauth/usage"

// usageUserAgent must look like the Claude Code client, otherwise the endpoint
// applies an aggressive per-token rate-limit bucket and returns 429s.
const usageUserAgent = "claude-code/1.0.60"

// authBySelector resolves an auth by auth_index, id, file name, label, or email.
func (h *Handler) authBySelector(sel string) *coreauth.Auth {
	sel = strings.TrimSpace(sel)
	if sel == "" || h == nil || h.authManager == nil {
		return nil
	}
	for _, a := range h.authManager.List() {
		if a == nil {
			continue
		}
		a.EnsureIndex()
		if strings.EqualFold(a.Index, sel) ||
			strings.EqualFold(a.ID, sel) ||
			strings.EqualFold(strings.TrimSuffix(a.ID, ".json"), sel) ||
			(a.FileName != "" && strings.EqualFold(a.FileName, sel)) ||
			(a.Label != "" && strings.EqualFold(a.Label, sel)) {
			return a
		}
		if a.Metadata != nil {
			if e, ok := a.Metadata["email"].(string); ok && e != "" && strings.EqualFold(e, sel) {
				return a
			}
		}
	}
	return nil
}

func claudeAccessToken(a *coreauth.Auth) string {
	if a == nil {
		return ""
	}
	if a.Metadata != nil {
		if v, ok := a.Metadata["access_token"].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	if a.Attributes != nil {
		return strings.TrimSpace(a.Attributes["api_key"])
	}
	return ""
}

// GetAccountUsage returns the Anthropic OAuth usage windows (five_hour,
// seven_day, seven_day_opus, seven_day_sonnet) for one Claude account, looked up
// by ?auth_index= / ?id= / ?account=. It calls Anthropic with the account's own
// OAuth access token, preferring the account's configured proxy/TLS and falling
// back to a direct request.
func (h *Handler) GetAccountUsage(c *gin.Context) {
	sel := strings.TrimSpace(c.Query("auth_index"))
	if sel == "" {
		sel = strings.TrimSpace(c.Query("id"))
	}
	if sel == "" {
		sel = strings.TrimSpace(c.Query("account"))
	}
	auth := h.authBySelector(sel)
	if auth == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "auth not found"})
		return
	}
	if !strings.EqualFold(strings.TrimSpace(auth.Provider), "claude") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "usage is only available for claude (anthropic) accounts"})
		return
	}
	token := claudeAccessToken(auth)
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "account has no oauth access token"})
		return
	}
	body, status, err := h.fetchAnthropicUsage(c.Request.Context(), auth, token)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	auth.EnsureIndex()
	c.Header("X-Account-Index", auth.Index)
	c.Data(status, "application/json", body)
}

func (h *Handler) fetchAnthropicUsage(ctx context.Context, auth *coreauth.Auth, token string) ([]byte, int, error) {
	do := func(client *http.Client) ([]byte, int, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, anthropicUsageURL, nil)
		if err != nil {
			return nil, 0, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("anthropic-beta", "oauth-2025-04-20")
		req.Header.Set("User-Agent", usageUserAgent)
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return b, resp.StatusCode, nil
	}
	// Prefer the account's client (proxy + TLS fingerprint). On transport error,
	// fall back to a plain direct request.
	if b, st, err := do(helps.NewUtlsHTTPClient(ctx, h.cfg, auth, 20*time.Second)); err == nil {
		return b, st, nil
	}
	return do(&http.Client{Timeout: 20 * time.Second})
}
