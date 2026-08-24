package image

import "testing"

func TestTagSHA(t *testing.T) {
	t.Parallel()
	cases := []struct {
		tag  string
		want string
	}{
		{"main-b8e5098", "b8e5098"},
		{"main-b8e509806517abcdef", "b8e509806517abcdef"},
		{"feature.x-deadbee", "deadbee"},
		{"main-dead", ""},
		{"main", ""},
		{"v1.2.3", ""},
		{"latest", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := TagSHA(tc.tag); got != tc.want {
			t.Fatalf("TagSHA(%q)=%q want %q", tc.tag, got, tc.want)
		}
	}
}

func TestParseGitHubRepo(t *testing.T) {
	t.Parallel()
	owner, repo, ok := ParseGitHubRepo("https://github.com/ianunruh/kmc")
	if !ok || owner != "ianunruh" || repo != "kmc" {
		t.Fatalf("got %s/%s ok=%v", owner, repo, ok)
	}
	owner, repo, ok = ParseGitHubRepo("https://www.github.com/ianunruh/kmc.git")
	if !ok || owner != "ianunruh" || repo != "kmc" {
		t.Fatalf(".git got %s/%s ok=%v", owner, repo, ok)
	}
	owner, repo, ok = ParseGitHubRepo("https://github.com/ianunruh/kmc/tree/main")
	if !ok || owner != "ianunruh" || repo != "kmc" {
		t.Fatalf("extra path got %s/%s ok=%v", owner, repo, ok)
	}
	if _, _, ok := ParseGitHubRepo("https://gitlab.com/ianunruh/kmc"); ok {
		t.Fatal("gitlab should not parse")
	}
	if _, _, ok := ParseGitHubRepo("git@github.com:ianunruh/kmc.git"); ok {
		t.Fatal("ssh should not parse")
	}
	if _, _, ok := ParseGitHubRepo(""); ok {
		t.Fatal("empty")
	}
}

func TestGitHubCommitURL(t *testing.T) {
	t.Parallel()
	got := GitHubCommitURL("https://github.com/ianunruh/kmc", "b8e5098")
	want := "https://github.com/ianunruh/kmc/commit/b8e5098"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if GitHubCommitURL("https://gitlab.com/ianunruh/kmc", "b8e5098") != "" {
		t.Fatal("gitlab url")
	}
	if GitHubCommitURL("https://github.com/ianunruh/kmc", "") != "" {
		t.Fatal("empty sha")
	}
}

func TestGitHubActionsURL(t *testing.T) {
	t.Parallel()
	got := GitHubActionsURL("https://github.com/ianunruh/kmc")
	want := "https://github.com/ianunruh/kmc/actions"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if GitHubActionsURL("https://gitlab.com/ianunruh/kmc") != "" {
		t.Fatal("gitlab url")
	}
	if GitHubActionsURL("") != "" {
		t.Fatal("empty")
	}
}
