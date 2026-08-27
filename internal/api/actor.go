package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/ianunruh/deploybot/internal/release"
)

const githubActionsIssuer = "token.actions.githubusercontent.com"

func actorFromRequest(r *http.Request) release.Actor {
	if r == nil {
		return release.Actor{}
	}
	raw := bearerToken(r.Header.Get("Authorization"))
	if raw == "" {
		return release.Actor{}
	}
	claims, err := decodeJWTPayload(raw)
	if err != nil {
		return release.Actor{}
	}
	return actorFromClaims(claims)
}

func bearerToken(h string) string {
	const prefix = "Bearer "
	if len(h) < len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(h[len(prefix):])
}

func decodeJWTPayload(token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil, errJWT
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	var claims map[string]any
	if err := json.Unmarshal(raw, &claims); err != nil {
		return nil, err
	}
	return claims, nil
}

var errJWT = errors.New("jwt")

func actorFromClaims(claims map[string]any) release.Actor {
	if isGitHubActions(claimString(claims, "iss")) {
		return release.Actor{
			Kind: release.ActorKindGitHubActions,
			ID:   claimString(claims, "actor"),
			Repo: claimString(claims, "repository"),
		}
	}
	id := claimString(claims, "preferred_username", "email", "name")
	email := claimString(claims, "email")
	name := claimString(claims, "name", "preferred_username")
	if id == "" && email == "" && name == "" {
		return release.Actor{}
	}
	if id == "" {
		id = name
	}
	return release.Actor{
		Kind:  release.ActorKindUser,
		ID:    id,
		Email: email,
		Name:  name,
	}
}

func isGitHubActions(iss string) bool {
	return strings.Contains(strings.ToLower(iss), githubActionsIssuer)
}

func claimString(claims map[string]any, keys ...string) string {
	for _, k := range keys {
		s, ok := claims[k].(string)
		if ok {
			s = strings.TrimSpace(s)
			if s != "" {
				return s
			}
		}
	}
	return ""
}
