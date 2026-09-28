// Package templatebuilderoauth implements the narrow OAuth authorization-code
// grant used by the CoBo Template Builder GPT Action.
package templatebuilderoauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	ScopeTemplateImportValidate = "cms.template.import.validate"
	CodeTTL                     = 5 * time.Minute
	AccessTokenTTL              = 10 * time.Minute
)

type ClientConfig struct {
	ClientID     string
	RedirectURIs []string
	SigningKey   []byte
}

type Subject struct {
	UserID       string
	MembershipID string
	CompanyID    string
}

type AuthorizationCode struct {
	CodeHash      string
	ClientID      string
	RedirectURI   string
	Scope         string
	CodeChallenge string
	Subject       Subject
	ExpiresAt     time.Time
	CreatedAt     time.Time
}

type GrantStore interface {
	CreateAuthorizationCode(ctx context.Context, code AuthorizationCode) error
	ConsumeAuthorizationCode(ctx context.Context, codeHash string, now time.Time) (*AuthorizationCode, error)
}

type Service struct {
	store  GrantStore
	client ClientConfig
	now    func() time.Time
}

func NewService(store GrantStore, client ClientConfig) (*Service, error) {
	if store == nil {
		return nil, fmt.Errorf("template builder oauth store is required")
	}
	client.ClientID = strings.TrimSpace(client.ClientID)
	if client.ClientID == "" || len(client.SigningKey) < 32 || len(client.RedirectURIs) == 0 {
		return nil, fmt.Errorf("template builder oauth configuration is incomplete")
	}
	for _, raw := range client.RedirectURIs {
		if !isHTTPSURL(raw) {
			return nil, fmt.Errorf("template builder oauth redirect URI must be HTTPS")
		}
	}
	return &Service{store: store, client: client, now: time.Now}, nil
}

type ApproveRequest struct {
	ClientID            string
	RedirectURI         string
	Scope               string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
	Subject             Subject
}

// ValidateAuthorizationRequest validates the public OAuth request before the
// browser is redirected to CoBo's consent screen.
func (s *Service) ValidateAuthorizationRequest(clientID, redirectURI, scope, challenge, method string) error {
	return s.validateAuthorizationRequest(clientID, redirectURI, scope, challenge, method)
}

func (s *Service) Approve(ctx context.Context, req ApproveRequest) (string, error) {
	if err := s.validateAuthorizationRequest(req.ClientID, req.RedirectURI, req.Scope, req.CodeChallenge, req.CodeChallengeMethod); err != nil {
		return "", err
	}
	if strings.TrimSpace(req.Subject.UserID) == "" || strings.TrimSpace(req.Subject.MembershipID) == "" || strings.TrimSpace(req.Subject.CompanyID) == "" {
		return "", fmt.Errorf("authenticated company context is required")
	}
	rawCode, err := randomToken(32)
	if err != nil {
		return "", err
	}
	now := s.now().UTC()
	if err := s.store.CreateAuthorizationCode(ctx, AuthorizationCode{
		CodeHash: sha256Hex(rawCode), ClientID: s.client.ClientID, RedirectURI: req.RedirectURI,
		Scope: ScopeTemplateImportValidate, CodeChallenge: req.CodeChallenge, Subject: req.Subject,
		CreatedAt: now, ExpiresAt: now.Add(CodeTTL),
	}); err != nil {
		return "", err
	}
	return appendAuthorizationCode(req.RedirectURI, rawCode, req.State)
}

type ExchangeRequest struct {
	ClientID     string
	Code         string
	RedirectURI  string
	CodeVerifier string
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
	Scope       string `json:"scope"`
}

func (s *Service) Exchange(ctx context.Context, req ExchangeRequest) (*TokenResponse, error) {
	if strings.TrimSpace(req.ClientID) != s.client.ClientID || !s.isAllowedRedirectURI(req.RedirectURI) {
		return nil, fmt.Errorf("invalid oauth client")
	}
	if strings.TrimSpace(req.Code) == "" || !validPKCEVerifier(req.CodeVerifier) {
		return nil, fmt.Errorf("invalid authorization-code exchange")
	}
	grant, err := s.store.ConsumeAuthorizationCode(ctx, sha256Hex(req.Code), s.now().UTC())
	if err != nil || grant == nil {
		return nil, fmt.Errorf("invalid or expired authorization code")
	}
	if grant.ClientID != s.client.ClientID || grant.RedirectURI != req.RedirectURI || grant.Scope != ScopeTemplateImportValidate || !pkceMatches(grant.CodeChallenge, req.CodeVerifier) {
		return nil, fmt.Errorf("invalid authorization-code exchange")
	}
	now := s.now().UTC()
	claims := actionClaims{
		Scope: ScopeTemplateImportValidate, MembershipID: grant.Subject.MembershipID, CompanyID: grant.Subject.CompanyID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: grant.Subject.UserID, Audience: []string{s.client.ClientID}, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(AccessTokenTTL)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.client.SigningKey)
	if err != nil {
		return nil, fmt.Errorf("issue action token: %w", err)
	}
	return &TokenResponse{AccessToken: token, TokenType: "Bearer", ExpiresIn: int64(AccessTokenTTL.Seconds()), Scope: ScopeTemplateImportValidate}, nil
}

func (s *Service) ValidateAccessToken(raw string) (Subject, error) {
	var claims actionClaims
	parsed, err := jwt.ParseWithClaims(strings.TrimSpace(raw), &claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return s.client.SigningKey, nil
	}, jwt.WithAudience(s.client.ClientID), jwt.WithLeeway(30*time.Second), jwt.WithTimeFunc(s.now))
	if err != nil || parsed == nil || !parsed.Valid || claims.Scope != ScopeTemplateImportValidate || strings.TrimSpace(claims.Subject) == "" || strings.TrimSpace(claims.MembershipID) == "" || strings.TrimSpace(claims.CompanyID) == "" {
		return Subject{}, fmt.Errorf("invalid action token")
	}
	return Subject{UserID: claims.Subject, MembershipID: claims.MembershipID, CompanyID: claims.CompanyID}, nil
}

func (s *Service) validateAuthorizationRequest(clientID, redirectURI, scope, challenge, method string) error {
	if strings.TrimSpace(clientID) != s.client.ClientID || !s.isAllowedRedirectURI(redirectURI) || strings.TrimSpace(scope) != ScopeTemplateImportValidate || method != "S256" || !validPKCEChallenge(challenge) {
		return fmt.Errorf("invalid authorization request")
	}
	return nil
}

func (s *Service) isAllowedRedirectURI(raw string) bool {
	for _, allowed := range s.client.RedirectURIs {
		if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(raw)), []byte(strings.TrimSpace(allowed))) == 1 {
			return true
		}
	}
	return false
}

type actionClaims struct {
	Scope        string `json:"scope"`
	MembershipID string `json:"membership_id"`
	CompanyID    string `json:"company_id"`
	jwt.RegisteredClaims
}

func randomToken(size int) (string, error) {
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("random oauth code: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func sha256Hex(raw string) string {
	s := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", s[:])
}

func pkceMatches(challenge, verifier string) bool {
	s := sha256.Sum256([]byte(verifier))
	actual := base64.RawURLEncoding.EncodeToString(s[:])
	return subtle.ConstantTimeCompare([]byte(actual), []byte(challenge)) == 1
}

func validPKCEChallenge(v string) bool { return len(v) >= 43 && len(v) <= 128 }
func validPKCEVerifier(v string) bool  { return len(v) >= 43 && len(v) <= 128 }

func isHTTPSURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && u.Scheme == "https" && u.Host != "" && u.Fragment == ""
}

func appendAuthorizationCode(rawRedirectURI, code, state string) (string, error) {
	u, err := url.Parse(rawRedirectURI)
	if err != nil {
		return "", fmt.Errorf("invalid redirect URI")
	}
	q := u.Query()
	q.Set("code", code)
	if strings.TrimSpace(state) != "" {
		q.Set("state", state)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}
