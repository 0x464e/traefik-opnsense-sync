package traefik

import (
	"context"
	"net/http"
	"strings"

	"github.com/0x464e/traefik-opnsense-sync/internal/httpx"
)

const routersApi = "/api/http/routers"

type Client interface {
	GetRouters(ctx context.Context) ([]Router, error)
}

type client struct {
	http     *http.Client
	baseURL  string
	username string
	password string
}

var _ Client = (*client)(nil)

func NewClient(baseURL string, verifyTls bool, username, password string) Client {
	return &client{
		http:     httpx.NewClient(verifyTls),
		baseURL:  strings.TrimRight(baseURL, "/"),
		username: username,
		password: password,
	}
}

func (c *client) GetRouters(ctx context.Context) ([]Router, error) {
	url := c.baseURL + routersApi

	var routers []Router

	if err := httpx.JsonRequest(ctx, c.http, http.MethodGet, url, nil, &routers, c.username, c.password); err != nil {
		return nil, err
	}
	return routers, nil
}
