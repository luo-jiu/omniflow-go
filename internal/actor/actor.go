package actor

type Kind string

const (
	KindAnonymous   Kind = "anonymous"
	KindUser        Kind = "user"
	KindAgent       Kind = "agent"
	KindSystem      Kind = "system"
	KindIntegration Kind = "integration"
)

type Actor struct {
	ID     string
	Name   string
	Kind   Kind
	Scopes []string
	Source string
}

func Anonymous() Actor {
	return Actor{Kind: KindAnonymous}
}

func System(id string) Actor {
	return Actor{
		ID:   id,
		Kind: KindSystem,
	}
}

func (a Actor) IsZero() bool {
	return a.ID == "" && a.Kind == ""
}
