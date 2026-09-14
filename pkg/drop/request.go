// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package drop

import (
	"fmt"
	"net/url"
	"strings"
)

// DropRepositoryURL is the home of the drop project, where community
// policy requests are filed.
const DropRepositoryURL = "https://github.com/carabiner-dev/drop"

// PolicyRequestTitlePrefix marks community policy request issues so they
// stand out in the drop repository's issue list.
const PolicyRequestTitlePrefix = "✨ Community Policies Request: "

// DefaultPolicyRepository returns the URL of the repository where drop looks
// for the policies of a publisher when no alternative source is configured:
// the organization's .github repository.
func DefaultPolicyRepository(host, org string) string {
	return fmt.Sprintf("https://%s/%s/%s", host, org, defaultPolicyRepo)
}

// PolicyPath returns the directory inside a policy repository holding the
// release policies of a repository.
func PolicyPath(repo string) string {
	return policyPathPrefix + "/" + repo
}

// PolicyRequest describes a request for the drop project to write community
// policies for an open source repository that has none.
type PolicyRequest struct {
	// Repository coordinates of the project lacking policies
	Host string
	Org  string
	Repo string

	// PolicyRepository is the URL where drop looked for policies
	PolicyRepository string

	// Release is the tag of the release drop checked for policies
	Release string

	// Version and Platform of the drop binary filing the request
	Version  string
	Platform string
}

// Slug returns the org/repo slug of the requested repository.
func (pr *PolicyRequest) Slug() string {
	return pr.Org + "/" + pr.Repo
}

// RepositoryURL returns the URL of the requested repository.
func (pr *PolicyRequest) RepositoryURL() string {
	return fmt.Sprintf("https://%s/%s/%s", pr.Host, pr.Org, pr.Repo)
}

// ReleaseURL returns the URL of the release that was checked, or an empty
// string when no release is recorded.
func (pr *PolicyRequest) ReleaseURL() string {
	if pr.Release == "" {
		return ""
	}
	return pr.RepositoryURL() + "/releases/tag/" + pr.Release
}

// Title returns the title of the request issue.
func (pr *PolicyRequest) Title() string {
	return PolicyRequestTitlePrefix + pr.Slug()
}

// Body returns the markdown body of the request issue.
func (pr *PolicyRequest) Body() string {
	policyRepo := pr.PolicyRepository
	if policyRepo == "" {
		policyRepo = DefaultPolicyRepository(pr.Host, pr.Org)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "## Community policies request\n\n")
	fmt.Fprintf(&b, "Please consider writing community policies for **%s** (%s).\n\n", pr.Slug(), pr.RepositoryURL())
	fmt.Fprintf(&b, "`drop` looked for policies in %s (under `%s/`) and found none, ", policyRepo, PolicyPath(pr.Repo))
	fmt.Fprintf(&b, "so the artifacts this project releases cannot be verified before installing them.\n\n")
	fmt.Fprintf(&b, "### Details\n\n")
	fmt.Fprintf(&b, "- Repository: %s\n", pr.RepositoryURL())
	fmt.Fprintf(&b, "- Policy source checked: %s (`%s/`)\n", policyRepo, PolicyPath(pr.Repo))
	if pr.Release != "" {
		fmt.Fprintf(&b, "- Release checked: [%s](%s)\n", pr.Release, pr.ReleaseURL())
	}
	if pr.Platform != "" {
		fmt.Fprintf(&b, "- Platform: %s\n", pr.Platform)
	}
	if pr.Version != "" {
		fmt.Fprintf(&b, "- drop version: %s\n", pr.Version)
	}
	fmt.Fprintf(&b, "\n<!-- Anything that helps is welcome: the release you tried to install, ")
	fmt.Fprintf(&b, "or the supply chain metadata the project publishes (SLSA provenance, SBOMs, signatures). -->\n")
	return b.String()
}

// URL returns the link to open a prefilled issue in the drop repository.
// Filing the issue happens in the browser, where the user is signed in to
// GitHub, so no token is required.
func (pr *PolicyRequest) URL() string {
	q := url.Values{}
	q.Set("title", pr.Title())
	q.Set("body", pr.Body())
	return DropRepositoryURL + "/issues/new?" + q.Encode()
}
