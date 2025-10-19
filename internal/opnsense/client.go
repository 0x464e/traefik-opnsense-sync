package opnsense

import (
	"context"
	"net/http"
	"strings"
	"traefik-opnsense-sync/internal/httpx"
	"traefik-opnsense-sync/internal/model"
)

const (
	searchHostOverrideApi = "/api/unbound/settings/search_host_override/"
	searchHostAliasApi    = "/api/unbound/settings/search_host_alias/"
	addHostAliasApi       = "/api/unbound/settings/add_host_alias/"
	deleteHostAliasApi    = "/api/unbound/settings/del_host_alias/"
	reconfigureApi        = "/api/unbound/service/reconfigure/"
)

type Client interface {
	getHostOverrides(ctx context.Context) ([]hostOverride, error)
	FindHostOverrideUUID(ctx context.Context, hostOverride string) (string, bool, error)
	GetHostAliases(ctx context.Context, hostOverrideUUID string) ([]model.HostAlias, error)
	AddHostAlias(ctx context.Context, alias model.HostAlias, hostOverrideUUID string) (string, error)
	DeleteHostAlias(ctx context.Context, alias model.HostAlias) error
	ReconfigureUnbound(ctx context.Context) error
}

type client struct {
	http      *http.Client
	baseURL   string
	apiKey    string
	apiSecret string
}

func NewClient(baseURL string, verifyTls bool, apiKey, apiSecret string) Client {
	return &client{
		http:      httpx.NewClient(verifyTls),
		baseURL:   strings.TrimRight(baseURL, "/"),
		apiKey:    apiKey,
		apiSecret: apiSecret,
	}
}

func (c *client) getHostOverrides(ctx context.Context) ([]hostOverride, error) {
	url := c.baseURL + searchHostOverrideApi

	var resp searchHostResponse
	if err := httpx.JsonRequest(ctx, c.http, http.MethodGet, url, nil, &resp, c.apiKey, c.apiSecret); err != nil {
		return nil, err
	}

	out := make([]hostOverride, 0, len(resp.Rows))
	for _, r := range resp.Rows {
		out = append(out, hostOverride{
			UUID:     r.UUID,
			Hostname: r.Hostname,
			Domain:   r.Domain,
		})
	}
	return out, nil
}

func (c *client) FindHostOverrideUUID(ctx context.Context, hostOverride string) (string, bool, error) {
	items, err := c.getHostOverrides(ctx)
	if err != nil {
		return "", false, err
	}
	for _, item := range items {
		if strings.EqualFold(item.Hostname+"."+item.Domain, hostOverride) {
			return item.UUID, true, nil
		}
	}
	return "", false, nil
}

func (c *client) GetHostAliases(ctx context.Context, hostOverrideUUID string) ([]model.HostAlias, error) {
	url := c.baseURL + searchHostAliasApi + hostOverrideUUID

	var resp searchHostResponse
	if err := httpx.JsonRequest(ctx, c.http, http.MethodGet, url, nil, &resp, c.apiKey, c.apiSecret); err != nil {
		return nil, err
	}

	out := make([]model.HostAlias, 0, len(resp.Rows))
	for _, r := range resp.Rows {
		out = append(out, model.HostAlias{
			UUID:        r.UUID,
			Hostname:    r.Hostname,
			Domain:      r.Domain,
			Description: r.Description,
		})
	}
	return out, nil
}

func (c *client) AddHostAlias(ctx context.Context, alias model.HostAlias, hostOverrideUUID string) (string, error) {
	url := c.baseURL + addHostAliasApi

	aliasCreate := hostAliasCreate{
		Enabled:     "1",
		Host:        hostOverrideUUID,
		Hostname:    alias.Hostname,
		Domain:      alias.Domain,
		Description: alias.Description,
	}

	type addReq struct {
		Alias hostAliasCreate `json:"alias"`
	}
	type addResp struct {
		Result string `json:"result"`
		UUID   string `json:"uuid,omitempty"`
	}

	var resp addResp
	if err := httpx.JsonRequest(ctx, c.http, http.MethodPost, url, addReq{Alias: aliasCreate}, &resp, c.apiKey, c.apiSecret); err != nil {
		return "", err
	}
	return resp.UUID, nil
}

func (c *client) DeleteHostAlias(ctx context.Context, alias model.HostAlias) error {
	url := c.baseURL + deleteHostAliasApi + alias.UUID

	if err := httpx.JsonRequest(ctx, c.http, http.MethodPost, url, nil, nil, c.apiKey, c.apiSecret); err != nil {
		return err
	}
	return nil
}

func (c *client) ReconfigureUnbound(ctx context.Context) error {
	url := c.baseURL + reconfigureApi

	if err := httpx.JsonRequest(ctx, c.http, http.MethodPost, url, nil, nil, c.apiKey, c.apiSecret); err != nil {
		return err
	}
	return nil
}
