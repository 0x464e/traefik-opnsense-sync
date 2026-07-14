package model

import "strings"

type OpKind int

const (
	OpCreate OpKind = iota
	OpDelete
)

func (o OpKind) String() string {
	switch o {
	case OpCreate:
		return "CREATE"
	case OpDelete:
		return "DELETE"
	default:
		return "UNKNOWN"
	}
}

type HostAlias struct {
	UUID        string
	Hostname    string
	Domain      string
	Description string
}

func (h *HostAlias) Key() string {
	return h.Hostname + "." + h.Domain
}

// NewHostAliasFromFQDN splits a fully-qualified domain name into a HostAlias's
// Hostname/Domain parts. It reports false if the FQDN has no dot separator or
// either resulting part would be empty.
func NewHostAliasFromFQDN(fqdn, description string) (HostAlias, bool) {
	hostname, domain, found := strings.Cut(fqdn, ".")
	if !found || hostname == "" || domain == "" {
		return HostAlias{}, false
	}
	return HostAlias{
		Hostname:    hostname,
		Domain:      domain,
		Description: description,
	}, true
}

type Operation struct {
	Kind  OpKind
	Alias HostAlias
}

type Plan struct {
	Operations []Operation
}

func (p *Plan) IsEmpty() bool {
	return len(p.Operations) == 0
}

func (p *Plan) AddOperation(kind OpKind, alias HostAlias) {
	p.Operations = append(p.Operations, Operation{
		Kind:  kind,
		Alias: alias,
	})
}
