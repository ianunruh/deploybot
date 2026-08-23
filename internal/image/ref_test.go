package image

import "testing"

func TestParse(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want Ref
	}{
		{
			in:   "ghcr.io/ianunruh/kmc",
			want: Ref{Repository: "ghcr.io/ianunruh/kmc"},
		},
		{
			in:   "ghcr.io/ianunruh/kmc:main",
			want: Ref{Repository: "ghcr.io/ianunruh/kmc", Tag: "main"},
		},
		{
			in:   "ghcr.io/ianunruh/kmc:main-abc1234@sha256:deadbeef",
			want: Ref{Repository: "ghcr.io/ianunruh/kmc", Tag: "main-abc1234", Digest: "sha256:deadbeef"},
		},
		{
			in:   "ghcr.io/ianunruh/kmc@sha256:deadbeef",
			want: Ref{Repository: "ghcr.io/ianunruh/kmc", Digest: "sha256:deadbeef"},
		},
		{
			in:   "localhost:5000/kmc:dev",
			want: Ref{Repository: "localhost:5000/kmc", Tag: "dev"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			got, err := Parse(tc.in)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got %+v want %+v", got, tc.want)
			}
			round, err := Parse(got.String())
			if err != nil {
				t.Fatal(err)
			}
			if round != got {
				t.Fatalf("round trip %+v -> %q -> %+v", got, got.String(), round)
			}
		})
	}
}

func TestParseEmpty(t *testing.T) {
	t.Parallel()
	if _, err := Parse("  "); err == nil {
		t.Fatal("expected error")
	}
}

func TestCompactAndLogName(t *testing.T) {
	t.Parallel()
	ref := MustParse("ghcr.io/ianunruh/kmc:main-b8e5098@sha256:a9514f5cd4b23e9f989f0c5348d47cc502727779f146007a8aae225fb7a06367")
	if got := ref.Compact(); got != "main-b8e5098@sha256:a9514f5cd4b2" {
		t.Fatalf("Compact %q", got)
	}
	if got := ref.LogName(); got != "ghcr.io/ianunruh/kmc:main-b8e5098" {
		t.Fatalf("LogName %q", got)
	}
	digestOnly := MustParse("ghcr.io/ianunruh/kmc@sha256:a9514f5cd4b23e9f989f0c5348d47cc502727779f146007a8aae225fb7a06367")
	if got := digestOnly.Compact(); got != "ghcr.io/ianunruh/kmc@sha256:a9514f5cd4b2" {
		t.Fatalf("digest Compact %q", got)
	}
	if got := digestOnly.LogName(); got != "ghcr.io/ianunruh/kmc@sha256:a9514f5cd4b2" {
		t.Fatalf("digest LogName %q", got)
	}
}
