package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type AuthManager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	users    map[string]string
	secret   []byte

	jwtSecret      []byte
	jwtExpiryHours int
	authToken      string
}

type Session struct {
	Token     string    `json:"token"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

func NewAuthManager() *AuthManager {
	secret := make([]byte, 32)
	rand.Read(secret)

	return &AuthManager{
		sessions: make(map[string]*Session),
		users:    make(map[string]string),
		secret:   secret,
	}
}

func (am *AuthManager) SetCredentials(username, password string) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.users[username] = password
}

func (am *AuthManager) ValidateUser(username, password string) bool {
	am.mu.RLock()
	defer am.mu.RUnlock()
	pass, ok := am.users[username]
	return ok && pass == password
}

func (am *AuthManager) CreateSession(username string) *Session {
	am.mu.Lock()
	defer am.mu.Unlock()

	token := am.generateToken(username)
	session := &Session{
		Token:     token,
		Username:  username,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	am.sessions[token] = session

	am.purgeExpired()
	return session
}

func (am *AuthManager) ValidateToken(token string) (*Session, bool) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	session, ok := am.sessions[token]
	if !ok {
		return nil, false
	}
	if time.Now().After(session.ExpiresAt) {
		return nil, false
	}
	return session, true
}

func (am *AuthManager) RevokeToken(token string) {
	am.mu.Lock()
	defer am.mu.Unlock()
	delete(am.sessions, token)
}

func (am *AuthManager) generateToken(username string) string {
	entropy := make([]byte, 16)
	rand.Read(entropy)
	hash := sha256.Sum256(append(entropy, []byte(username+time.Now().String())...))
	return hex.EncodeToString(hash[:])
}

func (am *AuthManager) purgeExpired() {
	now := time.Now()
	for token, session := range am.sessions {
		if now.After(session.ExpiresAt) {
			delete(am.sessions, token)
		}
	}
}

func (am *AuthManager) AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/login" || r.URL.Path == "/api/health" || r.URL.Path == "/api/auth/login" {
			next(w, r)
			return
		}

		token := am.extractToken(r)
		if token == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authorization required"})
			return
		}

		session, valid := am.ValidateToken(token)
		if !valid {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired token"})
			return
		}

		r.Header.Set("X-Username", session.Username)
		next(w, r)
	}
}

func (am *AuthManager) extractToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}

	cookie, err := r.Cookie("vesper_token")
	if err == nil {
		return cookie.Value
	}

	return r.URL.Query().Get("token")
}

func (am *AuthManager) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST only"})
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	if !am.ValidateUser(req.Username, req.Password) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	session := am.CreateSession(req.Username)

	http.SetCookie(w, &http.Cookie{
		Name:     "vesper_token",
		Value:    session.Token,
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		MaxAge:   86400,
		SameSite: http.SameSiteLaxMode,
	})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token":    session.Token,
		"username": session.Username,
		"expires":  session.ExpiresAt.Format(time.RFC3339),
		"role":     am.getRole(req.Username),
	})
}

func (am *AuthManager) HandleLogout(w http.ResponseWriter, r *http.Request) {
	token := am.extractToken(r)
	if token != "" {
		am.RevokeToken(token)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "vesper_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

func (am *AuthManager) getRole(username string) string {
	switch username {
	case "admin":
		return "administrator"
	case "operator":
		return "operator"
	case "viewer":
		return "viewer"
	default:
		return "operator"
	}
}

func (am *AuthManager) HandleMe(w http.ResponseWriter, r *http.Request) {
	token := am.extractToken(r)
	if token == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "no token"})
		return
	}

	session, valid := am.ValidateToken(token)
	if !valid {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"username": session.Username,
		"role":     am.getRole(session.Username),
		"expires":  session.ExpiresAt.Format(time.RFC3339),
	})
}

func base64URLEncode(data []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(data), "=")
}

func base64URLDecode(s string) ([]byte, error) {
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	return base64.URLEncoding.DecodeString(s)
}

func (am *AuthManager) generateJWT(subject string) (string, error) {
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	headerJSON, _ := json.Marshal(header)

	now := time.Now()
	expiry := now.Add(time.Duration(am.jwtExpiryHours) * time.Hour)

	payload := map[string]interface{}{
		"sub": subject,
		"iat": now.Unix(),
		"exp": expiry.Unix(),
	}
	payloadJSON, _ := json.Marshal(payload)

	headerEnc := base64URLEncode(headerJSON)
	payloadEnc := base64URLEncode(payloadJSON)
	signingInput := headerEnc + "." + payloadEnc

	mac := hmac.New(sha256.New, am.jwtSecret)
	mac.Write([]byte(signingInput))
	signature := mac.Sum(nil)
	signatureEnc := base64URLEncode(signature)

	return signingInput + "." + signatureEnc, nil
}

func (am *AuthManager) validateJWT(tokenStr string) (string, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid token format")
	}

	signingInput := parts[0] + "." + parts[1]
	signatureBytes, err := base64URLDecode(parts[2])
	if err != nil {
		return "", fmt.Errorf("invalid signature encoding")
	}

	mac := hmac.New(sha256.New, am.jwtSecret)
	mac.Write([]byte(signingInput))
	expectedSig := mac.Sum(nil)

	if !hmac.Equal(signatureBytes, expectedSig) {
		return "", fmt.Errorf("invalid signature")
	}

	payloadBytes, err := base64URLDecode(parts[1])
	if err != nil {
		return "", fmt.Errorf("invalid payload encoding")
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return "", fmt.Errorf("invalid payload JSON")
	}

	exp, ok := claims["exp"].(float64)
	if !ok {
		return "", fmt.Errorf("missing exp claim")
	}
	if time.Now().Unix() > int64(exp) {
		return "", fmt.Errorf("token expired")
	}

	sub, _ := claims["sub"].(string)
	return sub, nil
}

func (am *AuthManager) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST only"})
		return
	}

	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	if req.Token == "" || req.Token != am.authToken {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid auth token"})
		return
	}

	jwt, err := am.generateJWT("dashboard")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate token"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"jwt":     jwt,
		"expires": time.Now().Add(time.Duration(am.jwtExpiryHours) * time.Hour).Format(time.RFC3339),
	})
}

func (am *AuthManager) jwtAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if path == "/api/health" || path == "/api/auth/login" || path == "/ws" {
			next.ServeHTTP(w, r)
			return
		}

		if !strings.HasPrefix(path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing authorization header"})
			return
		}

		tokenStr := strings.TrimPrefix(auth, "Bearer ")
		sub, err := am.validateJWT(tokenStr)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
			return
		}

		r.Header.Set("X-JWT-Subject", sub)
		next.ServeHTTP(w, r)
	})
}

func (s *Server) SetupAuth() {
	s.auth = NewAuthManager()

	authToken := s.cfg.Dashboard.AuthToken
	jwtSecret := s.cfg.Dashboard.JWTSecret
	jwtExpiry := s.cfg.Dashboard.JWTExpiryHours

	if jwtExpiry <= 0 {
		jwtExpiry = 24
	}

	if jwtSecret == "" {
		secret := make([]byte, 32)
		rand.Read(secret)
		jwtSecret = hex.EncodeToString(secret)
	}

	s.auth.authToken = authToken
	s.auth.jwtSecret = []byte(jwtSecret)
	s.auth.jwtExpiryHours = jwtExpiry

	s.mux.HandleFunc("/api/auth/login", s.auth.handleAuthLogin)
	s.mux.HandleFunc("/api/login", s.auth.HandleLogin)
	s.mux.HandleFunc("/api/logout", s.auth.HandleLogout)
	s.mux.HandleFunc("/api/me", s.auth.HandleMe)

	if authToken != "" {
		s.srv.Handler = corsMiddleware(s.auth.jwtAuthMiddleware(s.mux))
		s.log.Info("JWT authentication enabled for dashboard API")
	} else {
		s.srv.Handler = corsMiddleware(s.mux)
		s.log.Info("Dashboard auth disabled (no auth_token configured)")
	}
}

func init() {
	_ = fmt.Sprintf("auth")
}
