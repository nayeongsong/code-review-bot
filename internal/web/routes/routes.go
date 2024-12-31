package routes

import (
	"context"

	"github.com/google/go-github/v68/github"
	"github.com/labstack/echo/v4"
	"github.com/openai/openai-go"
)

func CreateRoutes(e *echo.Echo, githubClient *github.Client, openAIClient *openai.Client, ctx context.Context) {
	githubController := &GithubController{GithubClient: githubClient, OpenAIClient: openAIClient, Context: ctx}

	e.GET("/repos/", githubController.ListAllRepositories)
	e.GET("/commits/repo/:repo/pulls/:pull_number/owner/:owner/", githubController.ListCommitsInPullRequest)
	e.GET("/patches/repo/:repo/pulls/:pull_number/owner/:owner/", githubController.ListPatchesInPullRequest)
	e.GET("/codereview/repo/:repo/pulls/:pull_number/owner/:owner/", githubController.GenerateCodeReview)
}
