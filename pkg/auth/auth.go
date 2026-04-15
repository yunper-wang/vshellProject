package auth

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
)

var (
	ErrAuthFailed   = errors.New("authentication failed")
	ErrInvalidToken = errors.New("invalid authentication token")
	ErrUnauthorized = errors.New("unauthorized")
	ErrTokenExpired = errors.New("token expired")
	ErrCertRevoked  = errors.New("certificate revoked")
)

// Authenticator provides authentication functionality
type Authenticator interface {
	// Authenticate validates client credentials
	Authenticate(creds Credentials) (Principal, error)

	// Authorize checks if principal has permission
	Authorize(principal Principal, permission string) bool
}

// Credentials represents authentication credentials
type Credentials struct {
	// Certificate for mTLS auth
	Certificate *x509.Certificate

	// Username/Password for token auth
	Username string
	Password string

	// Token for JWT/auth token
	Token string
}

// Principal represents an authenticated entity
type Principal struct {
	ID          string
	Type        string // "client", "service"
	Roles       []string
	Permissions []string
	Meta        map[string]string
}

// HasRole checks if principal has a role
func (p *Principal) HasRole(role string) bool {
	for _, r := range p.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasPermission checks if principal has a permission
func (p *Principal) HasPermission(perm string) bool {
	for _, p := range p.Permissions {
		if p == perm {
			return true
		}
	}
	return false
}

// MTLSAuthenticator authenticates via mutual TLS
type MTLSAuthenticator struct {
	trustedCAs *x509.CertPool
	verifyCB   func(*x509.Certificate) error
}

// NewMTLSAuthenticator creates a new mTLS authenticator
func NewMTLSAuthenticator(caPool *x509.CertPool, callback func(*x509.Certificate) error) *MTLSAuthenticator {
	return &MTLSAuthenticator{
		trustedCAs: caPool,
		verifyCB:   callback,
	}
}

// Authenticate validates TLS certificate
func (m *MTLSAuthenticator) Authenticate(creds Credentials) (Principal, error) {
	if creds.Certificate == nil {
		return Principal{}, ErrAuthFailed
	}

	// Verify certificate against CA
	opts := x509.VerifyOptions{
		Roots: m.trustedCAs,
	}

	chains, err := creds.Certificate.Verify(opts)
	if err != nil {
		return Principal{}, fmt.Errorf("%w: certificate verification failed: %v", ErrAuthFailed, err)
	}

	// Custom verification callback
	if m.verifyCB != nil {
		if err := m.verifyCB(creds.Certificate); err != nil {
			return Principal{}, err
		}
	}

	// Extract principal from certificate
	principal := Principal{
		ID:   creds.Certificate.Subject.CommonName,
		Type: "client",
		Meta: map[string]string{
			"serial": creds.Certificate.SerialNumber.String(),
			"issuer": creds.Certificate.Issuer.String(),
			"chains": fmt.Sprintf("%d", len(chains)),
		},
	}

	return principal, nil
}

// Authorize checks permission (for MTLS, permissions are typically in cert extensions)
func (m *MTLSAuthenticator) Authorize(principal Principal, permission string) bool {
	return principal.HasPermission(permission)
}

// TokenAuthenticator authenticates via tokens
type TokenAuthenticator struct {
	tokens map[string]*TokenInfo
}

// TokenInfo represents token metadata
type TokenInfo struct {
	Principal Principal
	ExpiresAt int64
	Revoked   bool
}

// NewTokenAuthenticator creates a new token authenticator
func NewTokenAuthenticator() *TokenAuthenticator {
	return &TokenAuthenticator{
		tokens: make(map[string]*TokenInfo),
	}
}

// Authenticate validates token
func (t *TokenAuthenticator) Authenticate(creds Credentials) (Principal, error) {
	if creds.Token == "" {
		return Principal{}, ErrInvalidToken
	}

	tokenInfo, ok := t.tokens[creds.Token]
	if !ok {
		return Principal{}, ErrInvalidToken
	}

	if tokenInfo.Revoked {
		return Principal{}, ErrCertRevoked
	}

	// Note: In production, check expiresAt against current time

	return tokenInfo.Principal, nil
}

// Authorize checks permission
func (t *TokenAuthenticator) Authorize(principal Principal, permission string) bool {
	return principal.HasPermission(permission)
}

// IssueToken creates a new token for a principal
func (t *TokenAuthenticator) IssueToken(principal Principal, expiresAt int64) string {
	// Simple token generation - in production use crypto/rand
	token := hex.EncodeToString([]byte(principal.ID + ":" + fmt.Sprintf("%d", expiresAt)))

	t.tokens[token] = &TokenInfo{
		Principal: principal,
		ExpiresAt: expiresAt,
		Revoked:   false,
	}

	return token
}

// RevokeToken revokes a token
func (t *TokenAuthenticator) RevokeToken(token string) error {
	tokenInfo, ok := t.tokens[token]
	if !ok {
		return ErrInvalidToken
	}

	tokenInfo.Revoked = true
	return nil
}

// GetPeerCertificate extracts certificate from TLS connection state
func GetPeerCertificate(state tls.ConnectionState) *x509.Certificate {
	if len(state.PeerCertificates) == 0 {
		return nil
	}
	return state.PeerCertificates[0]
}

// Permission constants
const (
	PermShellAccess  = "shell:access"
	PermShellCreate  = "shell:create"
	PermFileUpload   = "file:upload"
	PermFileDownload = "file:download"
	PermFileDelete   = "file:delete"
	PermAdmin        = "admin:full"
)

// Role constants
const (
	RoleAdmin    = "admin"
	RoleOperator = "operator"
	RoleReadOnly = "readonly"
)

// GetRolePermissions returns permissions for a role
func GetRolePermissions(role string) []string {
	switch role {
	case RoleAdmin:
		return []string{PermShellAccess, PermShellCreate, PermFileUpload, PermFileDownload, PermFileDelete, PermAdmin}
	case RoleOperator:
		return []string{PermShellAccess, PermShellCreate, PermFileUpload, PermFileDownload}
	case RoleReadOnly:
		return []string{PermShellAccess, PermFileDownload}
	default:
		return []string{}
	}
}
