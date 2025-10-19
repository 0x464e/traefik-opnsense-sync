package model

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
