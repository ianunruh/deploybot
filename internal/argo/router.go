package argo

// StaticRouter maps every stage to the same client (tests, single Argo).
type StaticRouter struct {
	Client Client
}

func (s StaticRouter) ForStage(string) Client { return s.Client }
