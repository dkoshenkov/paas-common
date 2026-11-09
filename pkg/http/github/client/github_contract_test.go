package client_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/dkoshenkov/paas-common/pkg/http/github/client"
	"github.com/dkoshenkov/paas-common/pkg/http/github/types"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGetKeysRoundTrip(t *testing.T) {
	link := `<https://api.github.com/users/octocat/keys?page=2>; rel="next"`
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet || r.URL.EscapedPath() != "/users/octo%20cat/keys" || r.URL.RawQuery != "page=1&per_page=100" {
			t.Fatalf("request = %s %s", r.Method, r.URL.String())
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Link": []string{link}}, Body: io.NopCloser(strings.NewReader(`[{"id":42,"key":"ssh-ed25519 AAAA"}]`)), Request: r}, nil
	})
	c, err := client.NewGithubClient("https://api.github.com", &http.Client{Transport: transport})
	if err != nil {
		t.Fatal(err)
	}
	perPage := 100
	response, err := c.GetKeys(context.Background(), types.KeysGithubGetKeysQuery{Username: "octo cat", Page: 1, PerPage: &perPage})
	if err != nil {
		t.Fatal(err)
	}
	result, ok := response.(client.GithubGetKeysResponse200)
	if !ok {
		t.Fatalf("response type = %T", response)
	}
	if len(result.Body) != 1 || result.Body[0].Id != 42 || result.Body[0].Key != "ssh-ed25519 AAAA" || result.Headers.Get("Link") != link {
		t.Fatalf("response = %#v", result)
	}
}
