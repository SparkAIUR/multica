package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type KeycloakClaims struct {
	Email  string
	Name   string
	Groups []string
}

type KeycloakClient struct {
	issuerURL    string
	clientID     string
	clientSecret string
	redirectURI  string
	httpClient   *http.Client
}

type keycloakDiscovery struct {
	TokenEndpoint string `json:"token_endpoint"`
	JWKSURI       string `json:"jwks_uri"`
}

type keycloakTokenResponse struct {
	AccessToken      string `json:"access_token"`
	IDToken          string `json:"id_token"`
	TokenType        string `json:"token_type"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type keycloakJWKSet struct {
	Keys []keycloakJWK `json:"keys"`
}

type keycloakJWK struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func keycloakIssuerURL() string {
	return strings.TrimRight(strings.TrimSpace(os.Getenv("KEYCLOAK_ISSUER_URL")), "/")
}

func KeycloakEnabled() bool {
	return keycloakIssuerURL() != "" && strings.TrimSpace(os.Getenv("KEYCLOAK_CLIENT_ID")) != ""
}

func NewKeycloakClientFromEnv() *KeycloakClient {
	issuerURL := keycloakIssuerURL()
	clientID := strings.TrimSpace(os.Getenv("KEYCLOAK_CLIENT_ID"))
	clientSecret := strings.TrimSpace(os.Getenv("KEYCLOAK_CLIENT_SECRET"))
	redirectURI := strings.TrimSpace(os.Getenv("KEYCLOAK_REDIRECT_URI"))
	if issuerURL == "" || clientID == "" {
		slog.Info("keycloak not configured", "issuer_set", issuerURL != "", "client_id_set", clientID != "")
		return nil
	}
	if clientSecret == "" {
		slog.Info("keycloak client secret not set, keycloak auth disabled")
		return nil
	}
	return &KeycloakClient{
		issuerURL:    issuerURL,
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
		httpClient:   &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *KeycloakClient) ExchangeCode(ctx context.Context, code, redirectURI string) (*KeycloakClaims, error) {
	if code == "" {
		return nil, fmt.Errorf("code is required")
	}
	if redirectURI == "" {
		redirectURI = c.redirectURI
	}
	if redirectURI == "" {
		return nil, fmt.Errorf("redirect URI is required")
	}

	discovery, err := c.fetchDiscovery(ctx)
	if err != nil {
		return nil, err
	}
	tokenResp, err := c.exchangeToken(ctx, discovery.TokenEndpoint, code, redirectURI)
	if err != nil {
		return nil, err
	}
	claims, err := c.verifyIDToken(ctx, discovery.JWKSURI, tokenResp.IDToken)
	if err != nil {
		return nil, err
	}
	return claims, nil
}

func (c *KeycloakClient) fetchDiscovery(ctx context.Context) (*keycloakDiscovery, error) {
	discoveryURL := c.issuerURL + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create keycloak discovery request: %w", err)
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("keycloak discovery request failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("keycloak discovery returned status %d", res.StatusCode)
	}

	var discovery keycloakDiscovery
	if err := json.NewDecoder(res.Body).Decode(&discovery); err != nil {
		return nil, fmt.Errorf("decode keycloak discovery: %w", err)
	}
	if discovery.TokenEndpoint == "" || discovery.JWKSURI == "" {
		return nil, fmt.Errorf("keycloak discovery missing required endpoints")
	}
	return &discovery, nil
}

func (c *KeycloakClient) exchangeToken(ctx context.Context, tokenEndpoint, code, redirectURI string) (*keycloakTokenResponse, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"redirect_uri":  {redirectURI},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create keycloak token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("keycloak token exchange failed: %w", err)
	}
	defer res.Body.Close()

	var tokenResp keycloakTokenResponse
	if err := json.NewDecoder(res.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("decode keycloak token response: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		if tokenResp.Error != "" {
			return nil, fmt.Errorf("keycloak token exchange failed: %s (%s)", tokenResp.Error, tokenResp.ErrorDescription)
		}
		return nil, fmt.Errorf("keycloak token exchange failed with status %d", res.StatusCode)
	}
	if tokenResp.IDToken == "" {
		return nil, fmt.Errorf("keycloak token response missing id_token")
	}
	return &tokenResp, nil
}

func (c *KeycloakClient) verifyIDToken(ctx context.Context, jwksURI, idToken string) (*KeycloakClaims, error) {
	keys, err := c.fetchJWKs(ctx, jwksURI)
	if err != nil {
		return nil, err
	}

	token, err := jwt.Parse(idToken, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, fmt.Errorf("id token missing kid header")
		}
		for _, jwk := range keys.Keys {
			if jwk.Kid == kid {
				return parseRSAPublicKey(jwk)
			}
		}
		return nil, fmt.Errorf("no matching key for kid %q", kid)
	}, jwt.WithValidMethods([]string{"RS256", "RS384", "RS512"}))
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid keycloak id token: %w", err)
	}

	mapClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid keycloak id token claims")
	}

	iss, err := mapClaims.GetIssuer()
	if err != nil || strings.TrimRight(iss, "/") != c.issuerURL {
		return nil, fmt.Errorf("keycloak issuer mismatch")
	}

	aud, err := mapClaims.GetAudience()
	if err != nil || !audContains(aud, c.clientID) {
		return nil, fmt.Errorf("keycloak audience mismatch")
	}

	email, _ := mapClaims["email"].(string)
	name, _ := mapClaims["name"].(string)
	groups := parseGroupsClaim(mapClaims["groups"])

	return &KeycloakClaims{
		Email:  strings.ToLower(strings.TrimSpace(email)),
		Name:   strings.TrimSpace(name),
		Groups: groups,
	}, nil
}

func (c *KeycloakClient) fetchJWKs(ctx context.Context, jwksURI string) (*keycloakJWKSet, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURI, nil)
	if err != nil {
		return nil, fmt.Errorf("create keycloak jwks request: %w", err)
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("keycloak jwks request failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("keycloak jwks returned status %d", res.StatusCode)
	}
	var keySet keycloakJWKSet
	if err := json.NewDecoder(res.Body).Decode(&keySet); err != nil {
		return nil, fmt.Errorf("decode keycloak jwks: %w", err)
	}
	if len(keySet.Keys) == 0 {
		return nil, fmt.Errorf("keycloak jwks has no keys")
	}
	return &keySet, nil
}

func parseRSAPublicKey(jwk keycloakJWK) (*rsa.PublicKey, error) {
	if jwk.Kty != "RSA" {
		return nil, fmt.Errorf("unsupported key type %q", jwk.Kty)
	}
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, fmt.Errorf("decode jwk modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, fmt.Errorf("decode jwk exponent: %w", err)
	}
	e := 0
	for _, b := range eBytes {
		e = (e << 8) | int(b)
	}
	if e == 0 {
		return nil, fmt.Errorf("invalid jwk exponent")
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: e,
	}, nil
}

func parseGroupsClaim(raw any) []string {
	switch v := raw.(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				continue
			}
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			out = append(out, s)
		}
		return out
	case []string:
		out := make([]string, 0, len(v))
		for _, s := range v {
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return nil
		}
		return []string{s}
	default:
		return nil
	}
}

func audContains(aud jwt.ClaimStrings, target string) bool {
	for _, value := range aud {
		if value == target {
			return true
		}
	}
	return false
}
