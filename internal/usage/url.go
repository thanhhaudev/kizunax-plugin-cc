package usage

import (
	"fmt"
	"net/url"
	"strings"
)

func usageURL(base string) string {
	return strings.TrimRight(base, "/") + "/api/coding/v1/usage"
}

func quotaURL(base string) string {
	return strings.TrimRight(base, "/") + "/api/v1/quota"
}

// DeriveBase extracts "scheme://host" from a config base URL, stripping any
// /api/... subpath. Returns an error if the URL has no scheme or host.
func DeriveBase(configBaseURL string) (string, error) {
	if configBaseURL == "" {
		return "", fmt.Errorf("empty base URL")
	}
	u, err := url.Parse(configBaseURL)
	if err != nil {
		return "", fmt.Errorf("parse: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("missing scheme or host in %q", configBaseURL)
	}
	return u.Scheme + "://" + u.Host, nil
}
