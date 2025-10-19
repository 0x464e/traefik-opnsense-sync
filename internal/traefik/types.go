package traefik

type Router struct {
	EntryPoints []string `json:"entryPoints"`
	Rule        string   `json:"rule"`
	Name        string   `json:"name"`
	Provider    string   `json:"provider"`
}

type DomainKind int

const (
	DomainLiteral DomainKind = iota
	DomainRegex
)

type DomainMatch struct {
	Value string
	Kind  DomainKind
}
