package httpx

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type errHTTP struct {
	StatusCode int
	Snippet    string
	URL        string
}

func NewClient(verifyTLS bool) *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: !verifyTLS,
			},
		},
	}
}

func (error *errHTTP) Error() string {
	return fmt.Sprintf("http %d (%s): %q", error.StatusCode, http.StatusText(error.StatusCode), error.URL)
}

func newRequest(method, rawUrl string, body io.Reader) (*http.Request, error) {
	if _, err := url.ParseRequestURI(rawUrl); err != nil {
		return nil, fmt.Errorf("invalid url %q: %w", rawUrl, err)
	}
	req, err := http.NewRequest(method, rawUrl, body)
	if err != nil {
		return nil, err
	}
	return req, nil
}

func request(ctx context.Context, cli *http.Client, req *http.Request) (*http.Response, error) {
	req = req.WithContext(ctx)

	resp, err := cli.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		return resp, nil
	}

	cutBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10)) // 4KB
	_ = resp.Body.Close()

	return nil, &errHTTP{
		StatusCode: resp.StatusCode,
		Snippet:    string(cutBody),
		URL:        req.URL.String(),
	}
}

func JsonRequest(ctx context.Context, cli *http.Client, method, rawURL string, in any, out any, basicUser, basicPass string) error {
	var body io.Reader
	if in != nil {
		buf, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("json marshal: %w", err)
		}
		body = bytes.NewReader(buf)
	}

	req, err := newRequest(method, rawURL, body)
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if basicUser != "" {
		req.SetBasicAuth(basicUser, basicPass)
	}

	resp, err := request(ctx, cli, req)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("json decode: %w", err)
	}
	return nil
}
