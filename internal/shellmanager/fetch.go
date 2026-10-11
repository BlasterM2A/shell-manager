package shellmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Fetcher downloads a URL into memory.
type Fetcher interface {
	Fetch(ctx context.Context, url string) ([]byte, error)
}

// HTTPFetcher fetches over HTTP. Token, if set, is sent only to the GitHub
// API (raises its rate limit).
type HTTPFetcher struct {
	Client *http.Client
	Token  string
}

func (f HTTPFetcher) Fetch(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if f.Token != "" && strings.HasPrefix(url, "https://api.github.com/") {
		req.Header.Set("Authorization", "Bearer "+f.Token)
	}
	client := f.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// latestTag returns the tag of a GitHub repo's latest release.
func latestTag(ctx context.Context, f Fetcher, repo string) (string, error) {
	data, err := f.Fetch(ctx, "https://api.github.com/repos/"+repo+"/releases/latest")
	if err != nil {
		return "", err
	}
	var rel struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(data, &rel); err != nil {
		return "", fmt.Errorf("parsing latest release of %s: %w", repo, err)
	}
	if rel.TagName == "" {
		return "", fmt.Errorf("latest release of %s has no tag", repo)
	}
	return rel.TagName, nil
}
