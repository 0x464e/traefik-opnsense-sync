package syncer

import (
	"testing"

	"github.com/0x464e/traefik-opnsense-sync/internal/model"
)

func TestEngineComputePlan(t *testing.T) {
	const tag = "Managed by traefik-opnsense-sync"

	alias := func(hostname, domain string) model.HostAlias {
		return model.HostAlias{Hostname: hostname, Domain: domain, Description: tag}
	}

	tests := []struct {
		name    string
		desired []model.HostAlias
		current []model.HostAlias
		want    []model.Operation
	}{
		{
			name:    "create only",
			desired: []model.HostAlias{alias("app", "example.com")},
			current: nil,
			want: []model.Operation{
				{Kind: model.OpCreate, Alias: alias("app", "example.com")},
			},
		},
		{
			name:    "delete only",
			desired: nil,
			current: []model.HostAlias{alias("app", "example.com")},
			want: []model.Operation{
				{Kind: model.OpDelete, Alias: alias("app", "example.com")},
			},
		},
		{
			name:    "no changes",
			desired: []model.HostAlias{alias("app", "example.com")},
			current: []model.HostAlias{alias("app", "example.com")},
			want:    nil,
		},
		{
			name:    "ignores untagged current aliases",
			desired: nil,
			current: []model.HostAlias{{Hostname: "app", Domain: "example.com", Description: "some other tool"}},
			want:    nil,
		},
		{
			name:    "mixed create and delete, ordered deletes-then-alphabetical",
			desired: []model.HostAlias{alias("b", "example.com"), alias("a", "example.com")},
			current: []model.HostAlias{alias("z", "example.com")},
			want: []model.Operation{
				{Kind: model.OpDelete, Alias: alias("z", "example.com")},
				{Kind: model.OpCreate, Alias: alias("a", "example.com")},
				{Kind: model.OpCreate, Alias: alias("b", "example.com")},
			},
		},
	}

	e := &Engine{descTag: tag}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := e.computePlan(tt.desired, tt.current)
			if err != nil {
				t.Fatalf("computePlan returned error: %v", err)
			}

			if len(plan.Operations) != len(tt.want) {
				t.Fatalf("got %d operations, want %d: %+v", len(plan.Operations), len(tt.want), plan.Operations)
			}
			for i, op := range plan.Operations {
				if op != tt.want[i] {
					t.Fatalf("operation %d = %+v, want %+v", i, op, tt.want[i])
				}
			}
		})
	}
}
