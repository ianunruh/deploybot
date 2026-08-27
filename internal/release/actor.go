package release

import (
	"strings"

	"github.com/ianunruh/deploybot/internal/gitwrite"
)

const (
	ActorKindAutoPin       = "auto-pin"
	ActorKindAutoPromote   = "auto-promote"
	ActorKindGitHubActions = "github-actions"
	ActorKindUser          = "user"

	trailerActor = "Deploybot-Actor"
	trailerID    = "Deploybot-Actor-ID"
	trailerRepo  = "Deploybot-Actor-Repo"
)

// Actor is who initiated a git mutation. Kind is the stable identifier
// written as a commit trailer; git author is the display form.
type Actor struct {
	Kind  string
	ID    string
	Repo  string
	Email string
	Name  string
}

func ActorAutoPin() Actor     { return Actor{Kind: ActorKindAutoPin} }
func ActorAutoPromote() Actor { return Actor{Kind: ActorKindAutoPromote} }

// GitAuthor is the commit author for this actor. Zero if Kind is empty so
// the process default author is used.
func (a Actor) GitAuthor() gitwrite.Author {
	if a.Kind == "" {
		return gitwrite.Author{}
	}
	name, email := a.ident()
	return gitwrite.Author{Name: name, Email: email}
}

func (a Actor) ident() (name, email string) {
	switch a.Kind {
	case ActorKindAutoPin:
		return ActorKindAutoPin, "auto-pin@kcloud.io"
	case ActorKindAutoPromote:
		return ActorKindAutoPromote, "auto-promote@kcloud.io"
	case ActorKindGitHubActions:
		name = ActorKindGitHubActions
		if repo := cleanIdent(a.Repo); repo != "" {
			name = ActorKindGitHubActions + "/" + repo
		}
		return name, "github-actions@kcloud.io"
	case ActorKindUser:
		name = firstIdent(a.ID, a.Name, a.Email)
		if name == "" {
			name = ActorKindUser
		}
		email = cleanIdent(a.Email)
		if email == "" {
			email = "user@kcloud.io"
		}
		return name, email
	default:
		name = firstIdent(a.ID, a.Kind)
		email = cleanIdent(a.Email)
		if email == "" {
			email = a.Kind + "@kcloud.io"
		}
		return name, email
	}
}

// AppendTrailers adds structured actor trailers after the subject. Empty
// Kind leaves the message unchanged so eventKind still matches the first line.
func (a Actor) AppendTrailers(message string) string {
	if a.Kind == "" {
		return message
	}
	var b strings.Builder
	b.WriteString(strings.TrimRight(message, "\n"))
	b.WriteString("\n\n")
	b.WriteString(trailerActor)
	b.WriteString(": ")
	b.WriteString(cleanIdent(a.Kind))
	if id := cleanIdent(a.ID); id != "" {
		b.WriteByte('\n')
		b.WriteString(trailerID)
		b.WriteString(": ")
		b.WriteString(id)
	}
	if repo := cleanIdent(a.Repo); repo != "" {
		b.WriteByte('\n')
		b.WriteString(trailerRepo)
		b.WriteString(": ")
		b.WriteString(repo)
	}
	return b.String()
}

func firstIdent(vals ...string) string {
	for _, v := range vals {
		if s := cleanIdent(v); s != "" {
			return s
		}
	}
	return ""
}

func cleanIdent(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

// WithActor returns a shallow copy that stamps this initiator on the next
// mutation. The process default service is left unchanged.
func (s *Service) WithActor(a Actor) *Service {
	if s == nil {
		return nil
	}
	s.cachesOnce()
	cp := *s
	cp.Actor = a
	return &cp
}
