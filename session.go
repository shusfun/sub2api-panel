package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"time"
)

const maxBody = 32 << 10
const cookieName = "__Host-sub2api-panel"

var signaturePattern = regexp.MustCompile(`^panel=\("@method" "@target-uri" "content-digest"\);created=([0-9]{1,12});nonce="([a-f0-9]{32})"$`)

func verifySignature(r *http.Request, origin string, key *ecdsa.PublicKey, now time.Time) (string, error) {
	invalid := fail(401, "invalid_proof")
	matches := signaturePattern.FindStringSubmatch(r.Header.Get("Signature-Input"))
	if len(matches) != 3 {
		return "", invalid
	}
	created, _ := strconv.ParseInt(matches[1], 10, 64)
	if delta := now.Unix() - created; delta < -10 || delta > 30 {
		return "", invalid
	}
	signature := r.Header.Get("Signature")
	if len(signature) != 96 || signature[:7] != "panel=:" || signature[len(signature)-1] != ':' {
		return "", invalid
	}
	raw, err := base64.StdEncoding.Strict().DecodeString(signature[7 : len(signature)-1])
	if err != nil || len(raw) != 64 {
		return "", invalid
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody+1))
	if err != nil || len(body) > maxBody {
		return "", fail(413, "invalid_input")
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	hash := sha256.Sum256(body)
	digest := "sha-256=:" + base64.StdEncoding.EncodeToString(hash[:]) + ":"
	if r.Header.Get("Content-Digest") != digest {
		return "", invalid
	}
	base := "\"@method\": " + r.Method + "\n\"@target-uri\": " + origin + r.URL.RequestURI() +
		"\n\"content-digest\": " + digest + "\n\"@signature-params\": " + matches[0][6:]
	message := sha256.Sum256([]byte(base))
	if !ecdsa.Verify(key, message[:], new(big.Int).SetBytes(raw[:32]), new(big.Int).SetBytes(raw[32:])) {
		return "", invalid
	}
	return matches[2], nil
}

func parseBrowserKey(value string) (*ecdsa.PublicKey, error) {
	raw, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err == nil && len(raw) == 65 {
		x, y := elliptic.Unmarshal(elliptic.P256(), raw)
		if x != nil {
			return &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}, nil
		}
	}
	return nil, fail(400, "invalid_browser_key")
}

type credentials struct {
	Access  string `json:"access_token"`
	Refresh string `json:"refresh_token"`
}

type identity struct {
	ID   int64  `json:"id"`
	Role string `json:"role"`
}

func currentUser(ctx context.Context, upstreamAPI serviceClient, token string) (identity, error) {
	user, err := upstream[identity](ctx, upstreamAPI, "GET", "/auth/me", token, "", "", nil)
	if err == nil && (user.ID < 1 || user.Role != "user" && user.Role != "admin") {
		err = fail(502, "invalid_upstream")
	}
	return user, err
}

type loginChallenge struct {
	expires time.Time
	key     string
	otp     string
}

type session struct {
	mu        sync.Mutex
	id        int64
	key       *ecdsa.PublicKey
	csrf      string
	created   time.Time
	last      time.Time
	nonces    map[string]time.Time
	encrypted []byte
	revoked   bool
}

type sessions struct {
	mu         sync.Mutex
	active     map[[32]byte]*session
	challenges map[string]loginChallenge
	vault      cipher.AEAD
}

func newSessions() *sessions {
	block, _ := aes.NewCipher(randomBytes(32))
	vault, _ := cipher.NewGCM(block)
	return &sessions{
		active: make(map[[32]byte]*session), challenges: make(map[string]loginChallenge), vault: vault,
	}
}

func (s *sessions) challenge(now time.Time, key, otp string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, pending := range s.challenges {
		if !pending.expires.After(now) {
			delete(s.challenges, id)
		}
	}
	if len(s.challenges) >= 4096 {
		return "", fail(429, "rate_limited")
	}
	id := randomToken()
	s.challenges[id] = loginChallenge{now.Add(2 * time.Minute), key, otp}
	return id, nil
}

func (s *sessions) takeChallenge(id string, now time.Time, key string, secondFactor bool) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pending, ok := s.challenges[id]
	if !ok || !pending.expires.After(now) || (pending.otp != "") != secondFactor || pending.key != "" && pending.key != key {
		return "", fail(401, "login_expired")
	}
	delete(s.challenges, id)
	return pending.otp, nil
}

func (s *sessions) seal(c credentials) []byte {
	data, _ := json.Marshal(c)
	nonce := randomBytes(s.vault.NonceSize())
	return s.vault.Seal(nonce, nonce, data, nil)
}

func (s *sessions) open(data []byte) (credentials, error) {
	var c credentials
	size := s.vault.NonceSize()
	decoded, err := s.vault.Open(nil, data[:size], data[size:], nil)
	if err == nil {
		err = json.Unmarshal(decoded, &c)
	}
	return c, err
}

func (s *sessions) create(now time.Time, key *ecdsa.PublicKey, user identity, c credentials) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, active := range s.active {
		active.mu.Lock()
		expired := !active.valid(now)
		active.mu.Unlock()
		if expired {
			delete(s.active, id)
		}
	}
	if len(s.active) >= 8192 {
		return "", fail(429, "rate_limited")
	}
	id := randomToken()
	s.active[sha256.Sum256([]byte(id))] = &session{
		id: user.ID, key: key, csrf: randomToken(), created: now, last: now,
		nonces: make(map[string]time.Time), encrypted: s.seal(c),
	}
	return id, nil
}

func (s *session) valid(now time.Time) bool {
	return !s.revoked && now.Sub(s.created) < 12*time.Hour && now.Sub(s.last) < 30*time.Minute
}

func (s *sessions) authenticate(r *http.Request, origin string, now time.Time) (*session, error) {
	cookie, err := r.Cookie(cookieName)
	if err != nil || len(cookie.Value) != 43 {
		return nil, fail(401, "sign_in_required")
	}
	s.mu.Lock()
	active := s.active[sha256.Sum256([]byte(cookie.Value))]
	s.mu.Unlock()
	if active == nil {
		return nil, fail(401, "sign_in_required")
	}
	nonce, err := verifySignature(r, origin, active.key, now)
	if err != nil {
		return nil, err
	}
	active.mu.Lock()
	defer active.mu.Unlock()
	if !active.valid(now) {
		return nil, fail(401, "sign_in_required")
	}
	if r.Method != "GET" && (r.Header.Get("Origin") != origin ||
		subtle.ConstantTimeCompare([]byte(active.csrf), []byte(r.Header.Get("X-CSRF-Token"))) != 1) {
		return nil, fail(403, "csrf_rejected")
	}
	for seen, expiry := range active.nonces {
		if !expiry.After(now) {
			delete(active.nonces, seen)
		}
	}
	if _, seen := active.nonces[nonce]; seen {
		return nil, fail(401, "replayed_proof")
	}
	if len(active.nonces) >= 2048 {
		return nil, fail(429, "rate_limited")
	}
	active.nonces[nonce] = now.Add(45 * time.Second)
	active.last = now
	return active, nil
}

func (s *sessions) principal(ctx context.Context, c serviceClient, active *session) (identity, credentials, error) {
	active.mu.Lock()
	defer active.mu.Unlock()
	if !active.valid(time.Now()) {
		return identity{}, credentials{}, fail(401, "sign_in_required")
	}
	tokens, err := s.open(active.encrypted)
	if err != nil {
		return identity{}, credentials{}, err
	}
	user, err := currentUser(ctx, c, tokens.Access)
	var status upstreamStatus
	if errors.As(err, &status) && status.status == 401 && tokens.Refresh != "" {
		refreshed, refreshErr := upstream[credentials](ctx, c, "POST", "/auth/refresh", "", "", "", struct {
			Refresh string `json:"refresh_token"`
		}{tokens.Refresh})
		if refreshErr != nil || refreshed.Access == "" || refreshed.Refresh == "" {
			if refreshErr == nil {
				refreshErr = fail(502, "invalid_upstream")
			}
			return identity{}, credentials{}, translateUpstream(refreshErr)
		}
		tokens = refreshed
		active.encrypted = s.seal(tokens)
		user, err = currentUser(ctx, c, tokens.Access)
	}
	if err != nil {
		return identity{}, credentials{}, translateUpstream(err)
	}
	if user.ID != active.id {
		active.revoked = true
		return identity{}, credentials{}, fail(401, "sign_in_required")
	}
	return user, tokens, nil
}
