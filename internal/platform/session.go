package platform

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
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"
)

const MaxBody = 32 << 10
const CookieName = "__Host-sub2api-panel"

var signaturePattern = regexp.MustCompile(`^panel=\("@method" "@target-uri" "content-digest"\);created=([0-9]{1,12});nonce="([a-f0-9]{32})"$`)

func VerifySignature(r *http.Request, origin string, key *ecdsa.PublicKey, now time.Time) (string, error) {
	return VerifySignatureWithLimit(r, origin, key, now, MaxBody)
}

func VerifySignatureWithLimit(r *http.Request, origin string, key *ecdsa.PublicKey, now time.Time, limit int64) (string, error) {
	invalid := Fail(401, "invalid_proof")
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
	digest := r.Header.Get("Content-Digest")
	if len(digest) != 54 {
		return "", invalid
	}
	base := "\"@method\": " + r.Method + "\n\"@target-uri\": " + origin + r.URL.RequestURI() +
		"\n\"content-digest\": " + digest + "\n\"@signature-params\": " + matches[0][6:]
	message := sha256.Sum256([]byte(base))
	if !ecdsa.Verify(key, message[:], new(big.Int).SetBytes(raw[:32]), new(big.Int).SetBytes(raw[32:])) {
		return "", invalid
	}
	actual, err := digestRequestBody(r, limit)
	if err != nil {
		return "", err
	}
	if actual != digest {
		return "", invalid
	}
	return matches[2], nil
}

type tempRequestBody struct{ *os.File }

func (f tempRequestBody) Close() error {
	return errors.Join(f.File.Close(), os.Remove(f.Name()))
}

func digestRequestBody(r *http.Request, limit int64) (string, error) {
	defer r.Body.Close()
	var hash []byte
	if limit == MaxBody {
		body, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
		if err != nil || int64(len(body)) > limit {
			return "", Fail(http.StatusRequestEntityTooLarge, "invalid_input")
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		sum := sha256.Sum256(body)
		hash = sum[:]
	} else {
		file, err := os.CreateTemp("", "sub2api-panel-upload-*")
		if err != nil {
			return "", err
		}
		temp := tempRequestBody{file}
		digest := sha256.New()
		n, err := io.Copy(io.MultiWriter(file, digest), io.LimitReader(r.Body, limit+1))
		if err != nil || n > limit {
			_ = temp.Close()
			return "", Fail(http.StatusRequestEntityTooLarge, "invalid_input")
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			_ = temp.Close()
			return "", err
		}
		r.Body = temp
		hash = digest.Sum(nil)
	}
	return "sha-256=:" + base64.StdEncoding.EncodeToString(hash) + ":", nil
}

func ParseBrowserKey(value string) (*ecdsa.PublicKey, error) {
	raw, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err == nil && len(raw) == 65 {
		x, y := elliptic.Unmarshal(elliptic.P256(), raw)
		if x != nil {
			return &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}, nil
		}
	}
	return nil, Fail(400, "invalid_browser_key")
}

type Credentials struct {
	Access  string `json:"access_token"`
	Refresh string `json:"refresh_token"`
}

type Identity struct {
	ID   int64  `json:"id"`
	Role string `json:"role"`
}

func CurrentUser(ctx context.Context, upstreamAPI ServiceClient, token string) (Identity, error) {
	user, err := Upstream[Identity](ctx, upstreamAPI, "GET", "/auth/me", token, "", "", nil)
	if err == nil && (user.ID < 1 || user.Role != "user" && user.Role != "admin") {
		err = Fail(502, "invalid_upstream")
	}
	return user, err
}

type loginChallenge struct {
	expires time.Time
	key     string
	otp     string
}

type Session struct {
	mu        sync.Mutex
	id        int64
	key       *ecdsa.PublicKey
	CSRF      string
	created   time.Time
	last      time.Time
	nonces    map[string]time.Time
	encrypted []byte
	revoked   bool
}

type Sessions struct {
	mu         sync.Mutex
	active     map[[32]byte]*Session
	challenges map[string]loginChallenge
	vault      cipher.AEAD
}

func NewSessions() *Sessions {
	block, _ := aes.NewCipher(RandomBytes(32))
	vault, _ := cipher.NewGCM(block)
	return &Sessions{
		active: make(map[[32]byte]*Session), challenges: make(map[string]loginChallenge), vault: vault,
	}
}

func (s *Sessions) Challenge(now time.Time, key, otp string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, pending := range s.challenges {
		if !pending.expires.After(now) {
			delete(s.challenges, id)
		}
	}
	if len(s.challenges) >= 4096 {
		return "", Fail(429, "rate_limited")
	}
	id := RandomToken()
	s.challenges[id] = loginChallenge{now.Add(2 * time.Minute), key, otp}
	return id, nil
}

func (s *Sessions) TakeChallenge(id string, now time.Time, key string, secondFactor bool) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pending, ok := s.challenges[id]
	if !ok || !pending.expires.After(now) || (pending.otp != "") != secondFactor || pending.key != "" && pending.key != key {
		return "", Fail(401, "login_expired")
	}
	delete(s.challenges, id)
	return pending.otp, nil
}

func (s *Sessions) seal(c Credentials) []byte {
	data, _ := json.Marshal(c)
	nonce := RandomBytes(s.vault.NonceSize())
	return s.vault.Seal(nonce, nonce, data, nil)
}

func (s *Sessions) open(data []byte) (Credentials, error) {
	var c Credentials
	size := s.vault.NonceSize()
	if len(data) < size {
		return c, Fail(401, "sign_in_required")
	}
	decoded, err := s.vault.Open(nil, data[:size], data[size:], nil)
	if err == nil {
		err = json.Unmarshal(decoded, &c)
	}
	return c, err
}

func (s *Sessions) Create(now time.Time, key *ecdsa.PublicKey, user Identity, c Credentials) (string, error) {
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
		return "", Fail(429, "rate_limited")
	}
	id := RandomToken()
	s.active[sha256.Sum256([]byte(id))] = &Session{
		id: user.ID, key: key, CSRF: RandomToken(), created: now, last: now,
		nonces: make(map[string]time.Time), encrypted: s.seal(c),
	}
	return id, nil
}

func (s *Session) valid(now time.Time) bool {
	return !s.revoked && now.Sub(s.created) < 12*time.Hour && now.Sub(s.last) < 30*time.Minute
}

func (s *Sessions) Revoke(active *Session) Credentials {
	active.mu.Lock()
	defer active.mu.Unlock()
	tokens, _ := s.open(active.encrypted)
	active.revoked = true
	active.encrypted = nil
	return tokens
}

func (s *Sessions) CSRFFor(id string) string {
	s.mu.Lock()
	active := s.active[sha256.Sum256([]byte(id))]
	s.mu.Unlock()
	if active == nil {
		return ""
	}
	return active.CSRF
}

func (s *Sessions) Authenticate(r *http.Request, origin string, now time.Time) (*Session, error) {
	return s.AuthenticateWithLimit(r, origin, now, MaxBody)
}

func (s *Sessions) AuthenticateWithLimit(r *http.Request, origin string, now time.Time, limit int64) (*Session, error) {
	cookie, err := r.Cookie(CookieName)
	if err != nil || len(cookie.Value) != 43 {
		return nil, Fail(401, "sign_in_required")
	}
	s.mu.Lock()
	active := s.active[sha256.Sum256([]byte(cookie.Value))]
	s.mu.Unlock()
	if active == nil {
		return nil, Fail(401, "sign_in_required")
	}
	nonce, err := VerifySignatureWithLimit(r, origin, active.key, now, limit)
	if err != nil {
		return nil, err
	}
	active.mu.Lock()
	defer active.mu.Unlock()
	if !active.valid(now) {
		return nil, Fail(401, "sign_in_required")
	}
	if r.Method != "GET" && (r.Header.Get("Origin") != origin ||
		subtle.ConstantTimeCompare([]byte(active.CSRF), []byte(r.Header.Get("X-CSRF-Token"))) != 1) {
		return nil, Fail(403, "csrf_rejected")
	}
	for seen, expiry := range active.nonces {
		if !expiry.After(now) {
			delete(active.nonces, seen)
		}
	}
	if _, seen := active.nonces[nonce]; seen {
		return nil, Fail(401, "replayed_proof")
	}
	if len(active.nonces) >= 2048 {
		return nil, Fail(429, "rate_limited")
	}
	active.nonces[nonce] = now.Add(45 * time.Second)
	active.last = now
	return active, nil
}

func (s *Sessions) Principal(ctx context.Context, c ServiceClient, active *Session) (Identity, Credentials, error) {
	active.mu.Lock()
	defer active.mu.Unlock()
	if !active.valid(time.Now()) {
		return Identity{}, Credentials{}, Fail(401, "sign_in_required")
	}
	tokens, err := s.open(active.encrypted)
	if err != nil {
		return Identity{}, Credentials{}, err
	}
	user, err := CurrentUser(ctx, c, tokens.Access)
	var status UpstreamStatus
	if errors.As(err, &status) && status.Status == 401 && tokens.Refresh != "" {
		refreshed, refreshErr := Upstream[Credentials](ctx, c, "POST", "/auth/refresh", "", "", "", struct {
			Refresh string `json:"refresh_token"`
		}{tokens.Refresh})
		if refreshErr != nil || refreshed.Access == "" || refreshed.Refresh == "" {
			if refreshErr == nil {
				refreshErr = Fail(502, "invalid_upstream")
			}
			return Identity{}, Credentials{}, TranslateUpstream(refreshErr)
		}
		tokens = refreshed
		active.encrypted = s.seal(tokens)
		user, err = CurrentUser(ctx, c, tokens.Access)
	}
	if err != nil {
		return Identity{}, Credentials{}, TranslateUpstream(err)
	}
	if user.ID != active.id {
		active.revoked = true
		return Identity{}, Credentials{}, Fail(401, "sign_in_required")
	}
	return user, tokens, nil
}

func SetSessionCookie(w http.ResponseWriter, id string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    id,
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   maxAge,
	})
}
