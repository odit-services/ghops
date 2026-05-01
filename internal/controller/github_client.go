package controller

import (
	"context"

	"github.com/google/go-github/v74/github"
)

// GitHubClient defines the GitHub API methods needed by the DeployKey controller.
type GitHubClient interface {
	CreateKey(ctx context.Context, owner, repo string, key *github.Key) (*github.Key, *github.Response, error)
	DeleteKey(ctx context.Context, owner, repo string, id int64) (*github.Response, error)
}

// githubClientAdapter wraps a *github.Client to implement GitHubClient.
type githubClientAdapter struct {
	client *github.Client
}

// CreateKey implements GitHubClient.
func (a *githubClientAdapter) CreateKey(ctx context.Context, owner, repo string, key *github.Key) (*github.Key, *github.Response, error) {
	return a.client.Repositories.CreateKey(ctx, owner, repo, key)
}

// DeleteKey implements GitHubClient.
func (a *githubClientAdapter) DeleteKey(ctx context.Context, owner, repo string, id int64) (*github.Response, error) {
	return a.client.Repositories.DeleteKey(ctx, owner, repo, id)
}

// NewGitHubClient creates a new GitHubClient adapter.
func NewGitHubClient(client *github.Client) GitHubClient {
	return &githubClientAdapter{client: client}
}
