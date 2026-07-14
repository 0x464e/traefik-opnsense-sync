package model

import "testing"

func TestNewHostAliasFromFQDN(t *testing.T) {
	tests := []struct {
		name        string
		fqdn        string
		description string
		wantAlias   HostAlias
		wantOK      bool
	}{
		{
			name:        "simple fqdn",
			fqdn:        "app.example.com",
			description: "tag",
			wantAlias:   HostAlias{Hostname: "app", Domain: "example.com", Description: "tag"},
			wantOK:      true,
		},
		{
			name:        "multi-label domain",
			fqdn:        "a.b.c.example.com",
			description: "tag",
			wantAlias:   HostAlias{Hostname: "a", Domain: "b.c.example.com", Description: "tag"},
			wantOK:      true,
		},
		{
			name:   "no dot",
			fqdn:   "localhost",
			wantOK: false,
		},
		{
			name:   "leading dot",
			fqdn:   ".example.com",
			wantOK: false,
		},
		{
			name:   "trailing dot",
			fqdn:   "app.",
			wantOK: false,
		},
		{
			name:   "empty",
			fqdn:   "",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := NewHostAliasFromFQDN(tt.fqdn, tt.description)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if got != tt.wantAlias {
				t.Fatalf("got %+v, want %+v", got, tt.wantAlias)
			}
		})
	}
}
