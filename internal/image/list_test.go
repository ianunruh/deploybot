package image

import "testing"

func TestCanonicalRepository(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"ghcr.io/ianunruh/kmc", "ghcr.io/ianunruh/kmc"},
		{"https://ghcr.io/ianunruh/kmc", "ghcr.io/ianunruh/kmc"},
		{"lscr.io/linuxserver/sonarr", "docker.io/linuxserver/sonarr"},
		{"LSCR.IO/linuxserver/sonarr/", "docker.io/linuxserver/sonarr"},
		{"linuxserver/sonarr", "docker.io/linuxserver/sonarr"},
		{"docker.io/linuxserver/sonarr", "docker.io/linuxserver/sonarr"},
		{"haugene/transmission-openvpn", "docker.io/haugene/transmission-openvpn"},
		{"nginx", "docker.io/library/nginx"},
		{"library/nginx", "docker.io/library/nginx"},
		{"docker.io/library/nginx", "docker.io/library/nginx"},
		{"index.docker.io/linuxserver/sonarr", "docker.io/linuxserver/sonarr"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := CanonicalRepository(tc.in); got != tc.want {
			t.Fatalf("CanonicalRepository(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseGHCR(t *testing.T) {
	t.Parallel()
	owner, name, err := parseGHCR("ghcr.io/ianunruh/kmc")
	if err != nil {
		t.Fatal(err)
	}
	if owner != "ianunruh" || name != "kmc" {
		t.Fatalf("got %s/%s", owner, name)
	}
	owner, name, err = parseGHCR("https://ghcr.io/ianunruh/kmc-controller")
	if err != nil {
		t.Fatal(err)
	}
	if owner != "ianunruh" || name != "kmc-controller" {
		t.Fatalf("got %s/%s", owner, name)
	}
	if _, _, err := parseGHCR("docker.io/library/nginx"); err == nil {
		t.Fatal("expected error")
	}
}

func TestPreferredTag(t *testing.T) {
	t.Parallel()
	cases := []struct {
		tags []string
		want string
	}{
		{[]string{"main", "latest", "main-b8e5098"}, "main-b8e5098"},
		{[]string{"latest"}, "latest"},
		{[]string{"v1.2.3", "latest"}, "v1.2.3"},
		{[]string{"main"}, "main"},
		{[]string{"testing", "testing-version-c4f9313", "26.2.20260821"}, "26.2.20260821"},
		{[]string{}, ""},
	}
	for _, tc := range cases {
		if got := PreferredTag(tc.tags); got != tc.want {
			t.Fatalf("PreferredTag(%v)=%q want %q", tc.tags, got, tc.want)
		}
	}
}

func TestResolveTokenPrefersDeploybotEnv(t *testing.T) {
	t.Setenv("DEPLOYBOT_GITHUB_TOKEN", "a")
	t.Setenv("GH_TOKEN", "b")
	t.Setenv("GITHUB_TOKEN", "c")
	token, src := ResolveToken()
	if token != "a" || src != "DEPLOYBOT_GITHUB_TOKEN" {
		t.Fatalf("got %s %s", token, src)
	}
}
