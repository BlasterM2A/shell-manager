package shellmanager

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPFetcher(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"), "token only goes to api.github.com")
		if r.URL.Path == "/ok" {
			_, _ = w.Write([]byte("body"))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	f := HTTPFetcher{Client: srv.Client(), Token: "secret"}

	data, err := f.Fetch(context.Background(), srv.URL+"/ok")
	require.NoError(t, err)
	assert.Equal(t, "body", string(data))

	_, err = f.Fetch(context.Background(), srv.URL+"/missing")
	assert.ErrorContains(t, err, "404")
}

func TestLatestTag(t *testing.T) {
	f := &fakeFetcher{data: map[string][]byte{
		"https://api.github.com/repos/o/r/releases/latest":     []byte(`{"tag_name":"v1.2.3"}`),
		"https://api.github.com/repos/o/empty/releases/latest": []byte(`{}`),
	}}
	tag, err := latestTag(context.Background(), f, "o/r")
	require.NoError(t, err)
	assert.Equal(t, "v1.2.3", tag)

	_, err = latestTag(context.Background(), f, "o/empty")
	assert.ErrorContains(t, err, "no tag")
}
