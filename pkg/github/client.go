// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	gogithub "github.com/google/go-github/v60/github"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

func New() (*Client, error) {
	token := os.Getenv("GITHUB_TOKEN")

	var httpClient *http.Client
	httpClient = http.DefaultClient
	if token != "" {
		httpClient = oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		))
	} else {
		logrus.Warn("Running unauthenticated. Watch out for rate limits from  the GitHub API")
	}

	client := gogithub.NewClient(httpClient)
	return &Client{
		Options: Options{
			Host: "github.com",
		},
		client: client,
	}, nil
}

type Options struct {
	Host string
}

type Client struct {
	Options Options
	client  *gogithub.Client
}

func NewAssetFromURLString(urlString string) *Asset {
	if strings.HasPrefix(urlString, "github.com") {
		urlString = "https://" + urlString
	}
	p, err := url.Parse(urlString)
	if err != nil {
		return nil
	}

	parts := strings.Split(strings.TrimPrefix(p.Path, "/"), "/")
	var org, repo, artifact, version string
	if len(parts) > 0 {
		org = parts[0]
	}
	if len(parts) > 1 {
		repo, _, _ = strings.Cut(parts[1], "@")
		// The version is expected in the last aprt of the oath
		_, version, _ = strings.Cut(parts[len(parts)-1], "@")
	}

	artifact = p.Fragment
	return &Asset{
		Host:    p.Hostname(),
		Org:     org,
		Repo:    repo,
		Version: version,
		Name:    artifact,
	}
}

// ListReleases returns a list of the latest releases in a repo
func (c *Client) ListReleases(rdata RepoDataProvider) ([]ReleaseDataProvider, error) {
	releases, _, err := c.client.Repositories.ListReleases(
		context.Background(), rdata.GetOrg(), rdata.GetRepo(), &gogithub.ListOptions{
			Page:    0,
			PerPage: 100,
		})
	if err != nil {
		return nil, fmt.Errorf("fetching release: %w", err)
	}

	ret := []ReleaseDataProvider{}
	for _, r := range releases {
		ret = append(ret, newReleaseFromGitHubRelease(rdata, r))
	}
	return ret, nil
}

func (c *Client) ListReleaseAssets(rdata ReleaseDataProvider) ([]AssetDataProvider, error) {
	releases, _, err := c.client.Repositories.ListReleases(
		context.Background(), rdata.GetOrg(), rdata.GetRepo(), &gogithub.ListOptions{
			Page:    0,
			PerPage: 100,
		})
	if err != nil {
		return nil, fmt.Errorf("fetching release: %w", err)
	}

	for _, r := range releases {
		if rdata.GetVersion() == "" || rdata.GetVersion() == "latest" {
			return buildReleaseAssets(rdata, r), nil
		}

		if rdata.GetVersion() == r.GetTagName() {
			return buildReleaseAssets(rdata, r), nil
		}
	}

	return nil, fmt.Errorf("release %v not found", rdata.GetVersion())
}

func buildReleaseAssets(src ReleaseDataProvider, release *gogithub.RepositoryRelease) []AssetDataProvider {
	ret := []AssetDataProvider{}
	for _, gha := range release.Assets {
		ret = append(ret, newAssetFromGitHubAsset(src, gha))
	}
	return ret
}
