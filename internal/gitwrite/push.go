package gitwrite

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

type PushResult struct {
	Remote string
	Branch string
	Commit string
}

func (p PushResult) Ref() string {
	if p.Remote == "" || p.Branch == "" {
		return ""
	}
	return p.Remote + "/" + p.Branch
}

// Push sends the current branch to its upstream (or origin). It never
// force-pushes. Already-up-to-date is success.
func Push(ctx context.Context, repoDir string) (PushResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	repo, err := git.PlainOpen(repoDir)
	if err != nil {
		return PushResult{}, fmt.Errorf("open repo %s: %w", repoDir, err)
	}
	head, err := repo.Head()
	if err != nil {
		return PushResult{}, fmt.Errorf("head: %w", err)
	}
	if !head.Name().IsBranch() {
		return PushResult{}, fmt.Errorf("refusing to push detached HEAD %s", head.Hash())
	}

	remoteName, err := pushRemote(repo, head.Name().Short())
	if err != nil {
		return PushResult{}, err
	}
	remote, err := repo.Remote(remoteName)
	if err != nil {
		return PushResult{}, fmt.Errorf("remote %s: %w", remoteName, err)
	}
	urls := remote.Config().URLs
	if len(urls) == 0 {
		return PushResult{}, fmt.Errorf("remote %s has no URL", remoteName)
	}

	spec := config.RefSpec(head.Name().String() + ":" + head.Name().String())
	if spec.IsForceUpdate() {
		return PushResult{}, fmt.Errorf("refusing force refspec %s", spec)
	}

	auth, err := authForURL(urls[len(urls)-1])
	if err != nil {
		return PushResult{}, err
	}

	err = repo.PushContext(ctx, &git.PushOptions{
		RemoteName: remoteName,
		RefSpecs:   []config.RefSpec{spec},
		Auth:       auth,
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return PushResult{}, fmt.Errorf("push %s to %s: %w", head.Name().Short(), remoteName, err)
	}
	return PushResult{
		Remote: remoteName,
		Branch: head.Name().Short(),
		Commit: head.Hash().String(),
	}, nil
}

func pushRemote(repo *git.Repository, branch string) (string, error) {
	cfg, err := repo.Config()
	if err != nil {
		return "", fmt.Errorf("config: %w", err)
	}
	if br, ok := cfg.Branches[branch]; ok && br.Remote != "" {
		return br.Remote, nil
	}
	if _, err := repo.Remote(git.DefaultRemoteName); err == nil {
		return git.DefaultRemoteName, nil
	}
	remotes, err := repo.Remotes()
	if err != nil {
		return "", fmt.Errorf("remotes: %w", err)
	}
	if len(remotes) == 1 {
		return remotes[0].Config().Name, nil
	}
	if len(remotes) == 0 {
		return "", fmt.Errorf("no remotes configured")
	}
	return "", fmt.Errorf("no upstream for %s and no origin remote", branch)
}

func authForURL(raw string) (transport.AuthMethod, error) {
	ep, err := transport.NewEndpoint(raw)
	if err != nil {
		return nil, fmt.Errorf("remote url %s: %w", raw, err)
	}
	switch ep.Protocol {
	case "file":
		return nil, nil
	case "ssh", "git":
		user := cmp.Or(ep.User, gitssh.DefaultUsername)
		return sshAuth(user)
	case "http", "https":
		token := gitToken()
		if token == "" {
			return nil, nil
		}
		user := cmp.Or(strings.TrimSpace(os.Getenv("DEPLOYBOT_GIT_USERNAME")), ep.User, "git")
		return &githttp.BasicAuth{Username: user, Password: token}, nil
	default:
		return nil, nil
	}
}

func sshAuth(user string) (transport.AuthMethod, error) {
	if auth, err := gitssh.NewSSHAgentAuth(user); err == nil {
		return auth, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("ssh auth: %w", err)
	}
	var errs []error
	for _, name := range []string{"id_ed25519", "id_rsa"} {
		path := filepath.Join(home, ".ssh", name)
		auth, err := gitssh.NewPublicKeysFromFile(user, path, "")
		if err == nil {
			return auth, nil
		}
		errs = append(errs, err)
	}
	return nil, fmt.Errorf("ssh auth: no agent and no default key: %w", errors.Join(errs...))
}

func gitToken() string {
	for _, e := range []string{"DEPLOYBOT_GIT_TOKEN", "DEPLOYBOT_GITHUB_TOKEN", "GH_TOKEN", "GITHUB_TOKEN"} {
		if v := strings.TrimSpace(os.Getenv(e)); v != "" {
			return v
		}
	}
	return ""
}
