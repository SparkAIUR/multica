package handler

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestKeycloakLogin_GroupPolicy(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	keyID := "test-key-1"
	clientID := "multica-web"
	clientSecret := "test-secret"
	redirectURI := "http://localhost:3000/auth/callback"
	issuer := ""

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"token_endpoint": issuer + "/protocol/openid-connect/token",
				"jwks_uri":       issuer + "/protocol/openid-connect/certs",
			})
		case "/protocol/openid-connect/certs":
			n := base64.RawURLEncoding.EncodeToString(privateKey.N.Bytes())
			e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privateKey.E)).Bytes())
			_ = json.NewEncoder(w).Encode(map[string]any{
				"keys": []map[string]string{{
					"kid": keyID,
					"kty": "RSA",
					"n":   n,
					"e":   e,
				}},
			})
		case "/protocol/openid-connect/token":
			_ = r.ParseForm()
			if r.FormValue("client_id") != clientID || r.FormValue("client_secret") != clientSecret {
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_client"})
				return
			}
			code := r.FormValue("code")
			var email, name string
			var groups any
			switch code {
			case "allowed":
				email = "keycloak-allowed@multica.ai"
				name = "Allowed User"
				groups = []string{"team-a"}
			case "denied":
				email = "keycloak-denied@multica.ai"
				name = "Denied User"
				groups = []string{"team-x"}
			case "nogroups":
				email = "keycloak-nogroups@multica.ai"
				name = "No Groups User"
			default:
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
				return
			}

			claims := jwt.MapClaims{
				"iss":   issuer,
				"aud":   []string{clientID},
				"exp":   time.Now().Add(time.Hour).Unix(),
				"iat":   time.Now().Unix(),
				"email": email,
				"name":  name,
			}
			if groups != nil {
				claims["groups"] = groups
			}

			token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
			token.Header["kid"] = keyID
			idToken, signErr := token.SignedString(privateKey)
			if signErr != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": signErr.Error()})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{
				"access_token": "access",
				"id_token":     idToken,
				"token_type":   "Bearer",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	issuer = server.URL

	t.Setenv("KEYCLOAK_ISSUER_URL", issuer)
	t.Setenv("KEYCLOAK_CLIENT_ID", clientID)
	t.Setenv("KEYCLOAK_CLIENT_SECRET", clientSecret)
	t.Setenv("KEYCLOAK_REDIRECT_URI", redirectURI)

	tests := []struct {
		name          string
		code          string
		allowedGroups string
		wantStatus    int
	}{
		{name: "allowed group", code: "allowed", allowedGroups: "team-a,team-b", wantStatus: http.StatusOK},
		{name: "denied group", code: "denied", allowedGroups: "team-a,team-b", wantStatus: http.StatusForbidden},
		{name: "empty allowed groups allows login", code: "denied", allowedGroups: "", wantStatus: http.StatusOK},
		{name: "missing groups denied when policy configured", code: "nogroups", allowedGroups: "team-a", wantStatus: http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("KEYCLOAK_ALLOWED_GROUPS", tt.allowedGroups)

			body := map[string]string{
				"code":         tt.code,
				"redirect_uri": redirectURI,
			}
			var buf bytes.Buffer
			_ = json.NewEncoder(&buf).Encode(body)
			req := httptest.NewRequest(http.MethodPost, "/auth/keycloak", &buf)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			testHandler.KeycloakLogin(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("KeycloakLogin: expected %d, got %d body=%s", tt.wantStatus, w.Code, w.Body.String())
			}

			// Clean up users created by successful test cases.
			if w.Code == http.StatusOK {
				var resp LoginResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("decode login response: %v", err)
				}
				if resp.User.Email == "" || resp.Token == "" {
					t.Fatalf("expected token + user email in response")
				}
				_, _ = testPool.Exec(context.Background(), `DELETE FROM "user" WHERE email = $1`, resp.User.Email)
			}
		})
	}
}
