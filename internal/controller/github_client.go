package controller

import (
	"context"

	"github.com/google/go-github/v93/github"
)

// GitHubClient defines the GitHub API methods needed by the DeployKey controller.
type GitHubClient interface {
	CreateKey(ctx context.Context, owner, repo string, req github.CreateDeployKeyRequest) (*github.Key, *github.Response, error)
	DeleteKey(ctx context.Context, owner, repo string, id int64) (*github.Response, error)
}

// githubClientAdapter wraps a *github.Client to implement GitHubClient.
type githubClientAdapter struct {
	client *github.Client
}

// CreateKey implements GitHubClient.
func (a *githubClientAdapter) CreateKey(ctx context.Context, owner, repo string, req github.CreateDeployKeyRequest) (*github.Key, *github.Response, error) {
	return a.client.Repositories.CreateKey(ctx, owner, repo, req)
}

// DeleteKey implements GitHubClient.
func (a *githubClientAdapter) DeleteKey(ctx context.Context, owner, repo string, id int64) (*github.Response, error) {
	return a.client.Repositories.DeleteKey(ctx, owner, repo, id)
}

// NewGitHubClient creates a new GitHubClient adapter.
func NewGitHubClient(client *github.Client) GitHubClient {
	return &githubClientAdapter{client: client}
}

// NewGitHubClientFromToken creates a GitHubClient authenticated with the given token.
func NewGitHubClientFromToken(token string) (GitHubClient, error) {
	client, err := github.NewClient(github.WithAuthToken(token))
	if err != nil {
		return nil, err
	}
	return NewGitHubClient(client), nil
}
