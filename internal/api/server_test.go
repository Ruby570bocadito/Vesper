package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ruby570bocadito/vesper/internal/orchestrator"
	"github.com/ruby570bocadito/vesper/pkg/shared/config"
	"github.com/ruby570bocadito/vesper/pkg/shared/types"
)

// newTestServer builds a real Server (routes + middleware chain) backed by a
// live orchestrator, and an httptest.Server exposing the same handler stack
// the production server would run: CORS -> rate limit -> (JWT) -> mux.
func newTestServer(t *testing.T, mutate func(*config.Config)) (*Server, *httptest.Server) {
	t.Helper()

	cfg := &config.Config{
		Logging: config.LoggingConfig{Level: "error", Format: "text"},
	}
	if mutate != nil {
		mutate(cfg)
	}

	orch, err := orchestrator.New(cfg)
	if err != nil {
		t.Fatalf("orchestrator.New: %v", err)
	}

	srv, err := New(cfg, orch)
	if err != nil {
		t.Fatalf("api.New: %v", err)
	}

	ts := httptest.NewServer(srv.srv.Handler)
	t.Cleanup(ts.Close)
	return srv, ts
}

// do sends a request and returns the response; the caller closes the body.
func do(t *testing.T, ts *httptest.Server, method, path, bearer string, body []byte) *http.Response {
	t.Helper()

	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, ts.URL+path, rdr)
	if err != nil {
		t.Fatalf("building request %s %s: %v", method, path, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return resp
}

func decodeBody(t *testing.T, resp *http.Response) map[string]interface{} {
	t.Helper()
	defer resp.Body.Close()

	var out map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decoding response body: %v", err)
	}
	return out
}

func decodeList(t *testing.T, resp *http.Response) []map[string]interface{} {
	t.Helper()
	defer resp.Body.Close()

	var out []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decoding response list: %v", err)
	}
	return out
}

// ============================================================
// HEALTH
// ============================================================

func TestHealthEndpoint(t *testing.T) {
	_, ts := newTestServer(t, nil)

	resp := do(t, ts, http.MethodGet, "/api/health", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health: got %d, want 200", resp.StatusCode)
	}
	body := decodeBody(t, resp)
	if body["status"] != "ok" {
		t.Fatalf("health status: got %v, want ok", body["status"])
	}
}

// ============================================================
// AUTH — session flow (cookie / bearer / query token)
// ============================================================

func TestLoginSuccessSetsCookie(t *testing.T) {
	srv, ts := newTestServer(t, nil)
	srv.auth.SetCredentials("admin", "s3cret")

	resp := do(t, ts, http.MethodPost, "/api/login", "",
		[]byte(`{"username":"admin","password":"s3cret"}`))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login: got %d, want 200", resp.StatusCode)
	}
	body := decodeBody(t, resp)
	if body["token"] == "" {
		t.Fatal("login response missing token")
	}
	if body["role"] != "administrator" {
		t.Fatalf("admin role: got %v", body["role"])
	}

	var cookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "vesper_token" {
			cookie = c
		}
	}
	if cookie == nil || cookie.Value == "" {
		t.Fatal("login did not set vesper_token cookie")
	}
	if !cookie.HttpOnly {
		t.Fatal("vesper_token cookie must be HttpOnly")
	}
}

func TestLoginInvalidCredentials(t *testing.T) {
	srv, ts := newTestServer(t, nil)
	srv.auth.SetCredentials("admin", "s3cret")

	resp := do(t, ts, http.MethodPost, "/api/login", "",
		[]byte(`{"username":"admin","password":"wrong"}`))
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad password: got %d, want 401", resp.StatusCode)
	}
}

func TestLoginMethodNotAllowed(t *testing.T) {
	_, ts := newTestServer(t, nil)

	resp := do(t, ts, http.MethodGet, "/api/login", "", nil)
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET /api/login: got %d, want 405", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestMeWithoutToken(t *testing.T) {
	_, ts := newTestServer(t, nil)

	resp := do(t, ts, http.MethodGet, "/api/me", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("/api/me without token: got %d, want 401", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestMeWithSessionToken(t *testing.T) {
	srv, ts := newTestServer(t, nil)
	srv.auth.SetCredentials("operator", "pw")

	login := do(t, ts, http.MethodPost, "/api/login", "",
		[]byte(`{"username":"operator","password":"pw"}`))
	body := decodeBody(t, login)
	token, _ := body["token"].(string)
	if token == "" {
		t.Fatal("no token from login")
	}

	me := do(t, ts, http.MethodGet, "/api/me", token, nil)
	if me.StatusCode != http.StatusOK {
		t.Fatalf("/api/me with token: got %d, want 200", me.StatusCode)
	}
	meBody := decodeBody(t, me)
	if meBody["username"] != "operator" {
		t.Fatalf("me username: got %v", meBody["username"])
	}
}

func TestLogoutRevokesSession(t *testing.T) {
	srv, ts := newTestServer(t, nil)
	srv.auth.SetCredentials("admin", "pw")

	login := do(t, ts, http.MethodPost, "/api/login", "",
		[]byte(`{"username":"admin","password":"pw"}`))
	token, _ := decodeBody(t, login)["token"].(string)

	logout := do(t, ts, http.MethodPost, "/api/logout", token, nil)
	if logout.StatusCode != http.StatusOK {
		t.Fatalf("logout: got %d, want 200", logout.StatusCode)
	}
	logout.Body.Close()

	if _, valid := srv.auth.ValidateToken(token); valid {
		t.Fatal("token still valid after logout")
	}

	me := do(t, ts, http.MethodGet, "/api/me", token, nil)
	if me.StatusCode != http.StatusUnauthorized {
		t.Fatalf("/api/me after logout: got %d, want 401", me.StatusCode)
	}
	me.Body.Close()
}

func TestSessionAcceptedViaCookieAndQuery(t *testing.T) {
	srv, ts := newTestServer(t, nil)
	srv.auth.SetCredentials("admin", "pw")

	session := srv.auth.CreateSession("admin")

	// cookie path
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/me", nil)
	req.AddCookie(&http.Cookie{Name: "vesper_token", Value: session.Token})
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("me via cookie: got %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()

	// query param path
	respQ := do(t, ts, http.MethodGet, "/api/me?token="+session.Token, "", nil)
	if respQ.StatusCode != http.StatusOK {
		t.Fatalf("me via query token: got %d, want 200", respQ.StatusCode)
	}
	respQ.Body.Close()
}

// ============================================================
// AUTH — JWT flow (dashboard auth_token mode)
// ============================================================

func jwtTestConfig(c *config.Config) {
	c.Dashboard.AuthToken = "dashboard-secret"
	c.Dashboard.JWTSecret = "unit-test-signing-key"
	c.Dashboard.JWTExpiryHours = 1
}

func TestJWTLoginAndProtectedRoute(t *testing.T) {
	_, ts := newTestServer(t, jwtTestConfig)

	// login with the dashboard token
	resp := do(t, ts, http.MethodPost, "/api/auth/login", "",
		[]byte(`{"token":"dashboard-secret"}`))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("auth login: got %d, want 200", resp.StatusCode)
	}
	jwtTok, _ := decodeBody(t, resp)["jwt"].(string)
	if jwtTok == "" || strings.Count(jwtTok, ".") != 2 {
		t.Fatalf("invalid JWT shape: %q", jwtTok)
	}

	// protected route with the JWT
	ok := do(t, ts, http.MethodGet, "/api/campaigns", jwtTok, nil)
	if ok.StatusCode != http.StatusOK {
		t.Fatalf("campaigns with jwt: got %d, want 200", ok.StatusCode)
	}
	ok.Body.Close()

	// no header
	anon := do(t, ts, http.MethodGet, "/api/campaigns", "", nil)
	if anon.StatusCode != http.StatusUnauthorized {
		t.Fatalf("campaigns anonymous: got %d, want 401", anon.StatusCode)
	}
	anon.Body.Close()

	// garbage token
	bad := do(t, ts, http.MethodGet, "/api/campaigns", "not-a-jwt", nil)
	if bad.StatusCode != http.StatusUnauthorized {
		t.Fatalf("campaigns bad jwt: got %d, want 401", bad.StatusCode)
	}
	bad.Body.Close()
}

func TestJWTExpiredRejected(t *testing.T) {
	srv, ts := newTestServer(t, jwtTestConfig)

	// negative expiry -> immediate expiration on next generated token
	srv.auth.mu.Lock()
	srv.auth.jwtExpiryHours = -1
	srv.auth.mu.Unlock()

	tok, err := srv.auth.generateJWT("dashboard")
	if err != nil {
		t.Fatal(err)
	}

	resp := do(t, ts, http.MethodGet, "/api/campaigns", tok, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expired jwt: got %d, want 401", resp.StatusCode)
	}
	body := decodeBody(t, resp)
	if body["error"] != "token expired" {
		t.Fatalf("expected expiry error, got %v", body["error"])
	}
}

func TestJWTTamperedPayloadRejected(t *testing.T) {
	srv, ts := newTestServer(t, jwtTestConfig)

	tok, err := srv.auth.generateJWT("dashboard")
	if err != nil {
		t.Fatal(err)
	}

	parts := strings.Split(tok, ".")
	tampered := parts[0] + "." + tamperBase64(parts[1]) + "." + parts[2]

	resp := do(t, ts, http.MethodGet, "/api/campaigns", tampered, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("tampered jwt: got %d, want 401", resp.StatusCode)
	}
	resp.Body.Close()
}

// tamperBase64 flips one character in a base64url segment.
func tamperBase64(s string) string {
	if s == "" {
		return "AAAA"
	}
	b := []byte(s)
	if b[0] == 'A' {
		b[0] = 'B'
	} else {
		b[0] = 'A'
	}
	return string(b)
}

func TestJWTLoginWrongToken(t *testing.T) {
	_, ts := newTestServer(t, jwtTestConfig)

	resp := do(t, ts, http.MethodPost, "/api/auth/login", "",
		[]byte(`{"token":"wrong-value"}`))
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("auth login wrong token: got %d, want 401", resp.StatusCode)
	}
	resp.Body.Close()
}

// ============================================================
// AGENTS
// ============================================================

func TestListAgentsEmpty(t *testing.T) {
	_, ts := newTestServer(t, nil)

	resp := do(t, ts, http.MethodGet, "/api/agents", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list agents: got %d, want 200", resp.StatusCode)
	}
	if agents := decodeList(t, resp); len(agents) != 0 {
		t.Fatalf("expected empty agent list, got %d", len(agents))
	}
}

func TestRegisterAndGetAgent(t *testing.T) {
	srv, ts := newTestServer(t, nil)

	agent := &types.Agent{
		ID:         "agt-1",
		CampaignID: "camp-1",
		Hostname:   "lab-host",
		OS:         "linux",
		Status:     types.AgentStatusActive,
	}
	srv.RegisterAgent(agent)

	resp := do(t, ts, http.MethodGet, "/api/agents/agt-1", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get agent: got %d, want 200", resp.StatusCode)
	}
	if body := decodeBody(t, resp); body["id"] != "agt-1" || body["hostname"] != "lab-host" {
		t.Fatalf("agent payload mismatch: %v", body)
	}

	missing := do(t, ts, http.MethodGet, "/api/agents/does-not-exist", "", nil)
	if missing.StatusCode != http.StatusNotFound {
		t.Fatalf("missing agent: got %d, want 404", missing.StatusCode)
	}
	missing.Body.Close()
}

func TestListAgentsFilterByCampaign(t *testing.T) {
	srv, ts := newTestServer(t, nil)

	srv.RegisterAgent(&types.Agent{ID: "a", CampaignID: "camp-1"})
	srv.RegisterAgent(&types.Agent{ID: "b", CampaignID: "camp-2"})

	resp := do(t, ts, http.MethodGet, "/api/agents?campaign_id=camp-1", "", nil)
	agents := decodeList(t, resp)
	if len(agents) != 1 || agents[0]["id"] != "a" {
		t.Fatalf("campaign filter: got %v", agents)
	}
}

func TestKillAgent(t *testing.T) {
	srv, ts := newTestServer(t, nil)
	srv.RegisterAgent(&types.Agent{ID: "victim", CampaignID: "camp-1"})

	resp := do(t, ts, http.MethodPost, "/api/agents/victim/kill", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("kill: got %d, want 200", resp.StatusCode)
	}
	if body := decodeBody(t, resp); body["success"] != true {
		t.Fatalf("kill success flag: %v", body)
	}

	if _, ok := srv.agents["victim"]; ok {
		t.Fatal("agent still registered after kill")
	}

	after := do(t, ts, http.MethodGet, "/api/agents/victim", "", nil)
	if after.StatusCode != http.StatusNotFound {
		t.Fatalf("killed agent: got %d, want 404", after.StatusCode)
	}
	after.Body.Close()
}

func TestAgentsPostNotAllowed(t *testing.T) {
	_, ts := newTestServer(t, nil)

	resp := do(t, ts, http.MethodPost, "/api/agents", "", []byte(`{}`))
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("POST /api/agents: got %d, want 405", resp.StatusCode)
	}
	resp.Body.Close()
}

// ============================================================
// CAMPAIGNS
// ============================================================

func TestListCampaignsEmpty(t *testing.T) {
	_, ts := newTestServer(t, nil)

	resp := do(t, ts, http.MethodGet, "/api/campaigns", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list campaigns: got %d, want 200", resp.StatusCode)
	}
	if camps := decodeList(t, resp); len(camps) != 0 {
		t.Fatalf("expected empty campaign list, got %d", len(camps))
	}
}

func TestCreateAndGetCampaign(t *testing.T) {
	_, ts := newTestServer(t, nil)

	resp := do(t, ts, http.MethodPost, "/api/campaigns", "",
		[]byte(`{"name":"lab-engage","target_scope":"127.0.0.0/8","goal":"validate detections","profile":"lab","auto_approve":true}`))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create campaign: got %d, want 201", resp.StatusCode)
	}
	body := decodeBody(t, resp)

	id, _ := body["id"].(string)
	if !strings.HasPrefix(id, "camp-") {
		t.Fatalf("campaign id shape: %q", id)
	}
	if body["status"] != string(types.CampaignStatusRunning) {
		t.Fatalf("new campaign status: %v", body["status"])
	}

	got := do(t, ts, http.MethodGet, "/api/campaigns/"+id, "", nil)
	if got.StatusCode != http.StatusOK {
		t.Fatalf("get campaign: got %d, want 200", got.StatusCode)
	}
	if gbody := decodeBody(t, got); gbody["name"] != "lab-engage" {
		t.Fatalf("campaign name round-trip: %v", gbody["name"])
	}
}

func TestPauseAndResumeCampaign(t *testing.T) {
	_, ts := newTestServer(t, nil)

	created := do(t, ts, http.MethodPost, "/api/campaigns", "",
		[]byte(`{"name":"pause-resume","target_scope":"127.0.0.1/32"}`))
	id, _ := decodeBody(t, created)["id"].(string)
	if id == "" {
		t.Fatal("no campaign id")
	}

	paused := do(t, ts, http.MethodPost, "/api/campaigns/"+id+"/pause", "", nil)
	if paused.StatusCode != http.StatusOK {
		t.Fatalf("pause: got %d, want 200", paused.StatusCode)
	}
	if b := decodeBody(t, paused); b["status"] != string(types.CampaignStatusPaused) {
		t.Fatalf("paused status: %v", b["status"])
	}

	resumed := do(t, ts, http.MethodPost, "/api/campaigns/"+id+"/resume", "", nil)
	if resumed.StatusCode != http.StatusOK {
		t.Fatalf("resume: got %d, want 200", resumed.StatusCode)
	}
	if b := decodeBody(t, resumed); b["status"] != string(types.CampaignStatusRunning) {
		t.Fatalf("resumed status: %v", b["status"])
	}
}

func TestGetCampaignNotFound(t *testing.T) {
	_, ts := newTestServer(t, nil)

	resp := do(t, ts, http.MethodGet, "/api/campaigns/camp-missing", "", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("missing campaign: got %d, want 404", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestCampaignsPutNotAllowed(t *testing.T) {
	_, ts := newTestServer(t, nil)

	resp := do(t, ts, http.MethodPut, "/api/campaigns", "", []byte(`{}`))
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("PUT /api/campaigns: got %d, want 405", resp.StatusCode)
	}
	resp.Body.Close()
}

// ============================================================
// CORS + RATE LIMIT
// ============================================================

func TestCORSHeadersOnGET(t *testing.T) {
	_, ts := newTestServer(t, nil)

	resp := do(t, ts, http.MethodGet, "/api/health", "", nil)
	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("missing CORS allow-origin header: %v", resp.Header)
	}
	resp.Body.Close()
}

func TestCORSOptionsPreflight(t *testing.T) {
	_, ts := newTestServer(t, nil)

	resp := do(t, ts, http.MethodOptions, "/api/campaigns", "", nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("OPTIONS preflight: got %d, want 204", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestRateLimitExceeded(t *testing.T) {
	_, ts := newTestServer(t, nil)

	saw429 := false
	for i := 0; i < 130; i++ { // bucket holds 100 tokens
		resp := do(t, ts, http.MethodGet, "/api/health", "", nil)
		if resp.StatusCode == http.StatusTooManyRequests {
			saw429 = true
		}
		resp.Body.Close()
		if saw429 {
			break
		}
	}
	if !saw429 {
		t.Fatal("expected a 429 once the token bucket was drained")
	}
}

// ============================================================
// UNIT — helpers
// ============================================================

func TestExtractID(t *testing.T) {
	cases := []struct {
		path, prefix, want string
	}{
		{"/api/agents/a1", "/api/agents/", "a1"},
		{"/api/agents/a1/kill", "/api/agents/", "a1"},
		{"/api/agents/a1/", "/api/agents/", "a1"},
		{"/api/agents/", "/api/agents/", ""},
		{"", "/api/agents/", ""},
	}
	for _, c := range cases {
		if got := extractID(c.path, c.prefix); got != c.want {
			t.Errorf("extractID(%q,%q) = %q, want %q", c.path, c.prefix, got, c.want)
		}
	}
}

func TestTokenBucketRefill(t *testing.T) {
	b := &tokenBucket{tokens: 0, maxTokens: 2, refillRate: 1000, lastRefill: time.Now()}
	if b.allow() {
		t.Fatal("empty bucket should not allow")
	}
	b.tokens = 2
	if !b.allow() {
		t.Fatal("full bucket should allow")
	}
	if b.tokens != 1 {
		t.Fatalf("tokens after allow = %v, want 1", b.tokens)
	}
}

func TestBase64URLDecodeRoundTrip(t *testing.T) {
	original := []byte(`{"sub":"dashboard","exp":123}`)
	enc := base64URLEncode(original)

	dec, err := base64URLDecode(enc)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !bytes.Equal(original, dec) {
		t.Fatalf("round trip mismatch: %q vs %q", original, dec)
	}
}
