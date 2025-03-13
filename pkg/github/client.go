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
	"time"

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
	}

	client := gogithub.NewClient(httpClient)
	return &Client{
		Options: Options{},
		client:  client,
	}, nil
}

type Options struct{}

type Client struct {
	Options Options
	client  *gogithub.Client
}

func NewAssetFromString(urlString string) *Asset {
	if strings.HasPrefix(urlString, "github.com") {
		urlString = "https://" + urlString
	}
	p, err := url.Parse(urlString)
	if err != nil {
		return nil
	}

	parts := strings.Split(strings.TrimPrefix(p.Path, "/"), "/")
	var org, repo, artifact, version string
	logrus.Infof("%+v", parts)
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
		Release: Release{
			RepoData: RepoData{
				Host: p.Hostname(),
				Org:  org,
				Repo: repo,
			},
			Version: version,
		},
		Name: artifact,
	}
}

type RepoDataProvider interface {
	GetHost() string
	GetRepo() string
	GetOrg() string
}

type ReleaseDataProvider interface {
	RepoDataProvider
	GetVersion() string
}

type Asset struct {
	Release
	Name        string
	DownloadURL string
	Author      string
	Size        int64
	Label       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type RepoData struct {
	Host string
	Repo string
	Org  string
}

func (r *RepoData) GetHost() string {
	return r.Host
}

func (r *RepoData) GetRepo() string {
	return r.Repo
}

func (r *Asset) GetOrg() string {
	return r.Org
}

type Release struct {
	RepoData
	Version string
}

func (r *Release) GetVersion() string {
	return r.Version
}

// ListReleases returns a list of the latest releases in a repo
func (c *Client) ListReleases(rdata RepoDataProvider) ([]*Release, error) {
	return nil, nil
}

func (c *Client) ListReleaseAsset(rdata ReleaseDataProvider) ([]*Asset, error) {
	releases, _, err := c.client.Repositories.ListReleases(
		context.Background(), rdata.GetOrg(), rdata.GetRepo(), &gogithub.ListOptions{
			Page:    0,
			PerPage: 100,
		})
	if err != nil {
		return nil, fmt.Errorf("fetching release: %w", err)
	}

	for _, r := range releases {
		if rdata.GetVersion() == "" {
			return buildReleaseAssets(rdata, r), nil
		}

		if rdata.GetVersion() == r.GetTagName() {
			return buildReleaseAssets(rdata, r), nil
		}
	}

	return nil, fmt.Errorf("release %v not found", rdata.GetVersion())
}

func buildReleaseAssets(src RepoDataProvider, release *gogithub.RepositoryRelease) []*Asset {
	ret := []*Asset{}
	for _, gha := range release.Assets {
		a := &Asset{
			Release: Release{
				RepoData: RepoData{
					Host: src.GetHost(),
					Org:  src.GetOrg(),
					Repo: src.GetRepo(),
				},
				Version: release.GetTagName(),
			},
			Name:        gha.GetName(),
			DownloadURL: gha.GetBrowserDownloadURL(),
			Author:      gha.GetUploader().GetLogin(),
			CreatedAt:   *gha.CreatedAt.GetTime(),
			UpdatedAt:   *gha.UpdatedAt.GetTime(),
			Size:        int64(gha.GetSize()),
			Label:       gha.GetLabel(),
		}
		ret = append(ret, a)
	}
	return ret
}
