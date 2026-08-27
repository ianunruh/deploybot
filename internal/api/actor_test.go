package api

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ianunruh/deploybot/internal/release"
)

func TestActorFromGitHubJWT(t *testing.T) {
	t.Parallel()
	tok := unsignedJWT(map[string]any{
		"iss":        "https://token.actions.githubusercontent.com",
		"actor":      "ianunruh",
		"repository": "ianunruh/kmc",
		"workflow":   "Build and Push Docker Images",
	})
	r := httptest.NewRequest(http.MethodPost, "/pin", nil)
	r.Header.Set("Authorization", "Bearer "+tok)
	got := actorFromRequest(r)
	if got.Kind != release.ActorKindGitHubActions || got.ID != "ianunruh" || got.Repo != "ianunruh/kmc" {
		t.Fatalf("%+v", got)
	}
}

func TestActorFromDexJWT(t *testing.T) {
	t.Parallel()
	tok := unsignedJWT(map[string]any{
		"iss":                "https://dex.k8s.kcloud.io",
		"email":              "ian@kcloud.io",
		"name":               "Ian Unruh",
		"preferred_username": "ianunruh",
	})
	r := httptest.NewRequest(http.MethodPost, "/pin", nil)
	r.Header.Set("Authorization", "bearer "+tok)
	got := actorFromRequest(r)
	if got.Kind != release.ActorKindUser || got.ID != "ianunruh" || got.Email != "ian@kcloud.io" {
		t.Fatalf("%+v", got)
	}
}

func TestActorFromRequestFallback(t *testing.T) {
	t.Parallel()
	if got := actorFromRequest(httptest.NewRequest(http.MethodPost, "/pin", nil)); got.Kind != "" {
		t.Fatalf("%+v", got)
	}
	r := httptest.NewRequest(http.MethodPost, "/pin", nil)
	r.Header.Set("Authorization", "Bearer not-a-jwt")
	if got := actorFromRequest(r); got.Kind != "" {
		t.Fatalf("garbage jwt %+v", got)
	}
	r2 := httptest.NewRequest(http.MethodPost, "/pin", nil)
	r2.Header.Set("Authorization", "Bearer "+unsignedJWT(map[string]any{"iss": "https://example"}))
	if got := actorFromRequest(r2); got.Kind != "" {
		t.Fatalf("unknown jwt %+v", got)
	}
}

func TestMutateWithGitHubActor(t *testing.T) {
	t.Parallel()
	s := &Server{Release: &release.Service{}}
	r := httptest.NewRequest(http.MethodPost, "/pin", nil)
	r.Header.Set("Authorization", "Bearer "+unsignedJWT(map[string]any{
		"iss":        "https://token.actions.githubusercontent.com",
		"actor":      "ianunruh",
		"repository": "ianunruh/humpty",
	}))
	got := s.mutateWith(r, nil, nil)
	if got.Actor.Kind != release.ActorKindGitHubActions || got.Actor.Repo != "ianunruh/humpty" {
		t.Fatalf("%+v", got.Actor)
	}
	if got == s.Release {
		t.Fatal("should copy")
	}
	if s.Release.Actor.Kind != "" {
		t.Fatal("process default mutated")
	}
}

func TestMutateWithNoActor(t *testing.T) {
	t.Parallel()
	s := &Server{Release: &release.Service{Sync: true}}
	got := s.mutateWith(httptest.NewRequest(http.MethodPost, "/pin", nil), nil, nil)
	if got.Actor.Kind != "" {
		t.Fatalf("%+v", got.Actor)
	}
	if got != s.Release {
		t.Fatal("omitted actor should keep the process default")
	}
}

func unsignedJWT(claims map[string]any) string {
	hdr := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	body, err := json.Marshal(claims)
	if err != nil {
		panic(err)
	}
	return hdr + "." + base64.RawURLEncoding.EncodeToString(body) + ".x"
}
