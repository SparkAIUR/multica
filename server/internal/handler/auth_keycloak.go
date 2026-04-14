package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/logger"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type KeycloakLoginRequest struct {
	Code        string `json:"code"`
	RedirectURI string `json:"redirect_uri"`
}

func (h *Handler) AuthMethodDisabled(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotFound, "authentication method disabled")
}

func (h *Handler) KeycloakLogin(w http.ResponseWriter, r *http.Request) {
	var req KeycloakLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Code) == "" {
		writeError(w, http.StatusBadRequest, "code is required")
		return
	}

	kc := auth.NewKeycloakClientFromEnv()
	if kc == nil {
		writeError(w, http.StatusServiceUnavailable, "Keycloak login is not configured")
		return
	}

	redirectURI := strings.TrimSpace(req.RedirectURI)
	if redirectURI == "" {
		redirectURI = strings.TrimSpace(os.Getenv("KEYCLOAK_REDIRECT_URI"))
	}

	claims, err := kc.ExchangeCode(r.Context(), strings.TrimSpace(req.Code), redirectURI)
	if err != nil {
		slog.Warn("keycloak login failed", append(logger.RequestAttrs(r), "error", err)...)
		writeError(w, http.StatusUnauthorized, "failed to authenticate with Keycloak")
		return
	}
	if claims.Email == "" {
		writeError(w, http.StatusBadRequest, "Keycloak account has no email")
		return
	}

	allowedGroups := auth.AllowedKeycloakGroupsFromEnv()
	if !auth.IsAnyGroupAllowed(claims.Groups, allowedGroups) {
		slog.Info("keycloak login denied by group policy", append(logger.RequestAttrs(r), "email", claims.Email, "groups", claims.Groups)...)
		writeError(w, http.StatusForbidden, "access denied by group policy")
		return
	}

	user, err := h.findOrCreateUser(r.Context(), claims.Email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	// If the account was auto-created with email-prefix name, prefer Keycloak profile name.
	if claims.Name != "" && user.Name == strings.Split(claims.Email, "@")[0] {
		updated, updateErr := h.Queries.UpdateUser(r.Context(), db.UpdateUserParams{
			ID:   user.ID,
			Name: claims.Name,
		})
		if updateErr == nil {
			user = updated
		}
	}

	tokenString, err := h.issueJWT(user)
	if err != nil {
		slog.Warn("keycloak login failed to issue jwt", append(logger.RequestAttrs(r), "error", err, "email", claims.Email)...)
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	if err := auth.SetAuthCookies(w, tokenString); err != nil {
		slog.Warn("failed to set auth cookies", "error", err)
	}
	if h.CFSigner != nil {
		for _, cookie := range h.CFSigner.SignedCookies(time.Now().Add(72 * time.Hour)) {
			http.SetCookie(w, cookie)
		}
	}

	slog.Info("user logged in via keycloak", append(logger.RequestAttrs(r), "user_id", uuidToString(user.ID), "email", user.Email)...)
	writeJSON(w, http.StatusOK, LoginResponse{
		Token: tokenString,
		User:  userToResponse(user),
	})
}
