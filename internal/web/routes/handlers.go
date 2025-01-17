package routes

import (
	"code-review-bot/internal/domain"
	"code-review-bot/internal/web"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/go-github/v68/github"
	"github.com/labstack/echo/v4"
	"github.com/openai/openai-go"
	"go.uber.org/zap"
)

type GithubController struct {
	GithubClient *github.Client
	OpenAIClient *openai.Client
	Context      context.Context
}

func (controller *GithubController) ListAllRepositories(e echo.Context) error {
	cc := e.(*web.AppContext)

	repos, _, err := controller.GithubClient.Repositories.ListByUser(controller.Context, "nayeongsong", nil)
	if err != nil {
		cc.AppLogger.Error("Failed to fetch repositories", zap.Error(err))
		return err
	}

	return e.JSON(http.StatusOK, repos)
}

func (controller *GithubController) ListCommitsInPullRequest(e echo.Context) error {
	cc := e.(*web.AppContext)

	repoName := e.Param("repo")
	prNumber64, err := strconv.ParseInt(e.Param("pull_number"), 10, 64)
	if err != nil {
		cc.AppLogger.Error("Failed to parse PR number", zap.Error(err))
		return err
	}
	prNumber := int(prNumber64)
	owner := e.Param("owner")

	commits, _, err := controller.GithubClient.PullRequests.ListCommits(
		controller.Context,
		owner,
		repoName,
		prNumber,
		nil,
	)
	if err != nil {
		cc.AppLogger.Error("Failed to fetch commits", zap.Error(err))
		return err
	}

	return e.JSON(http.StatusOK, commits)
}

func (controller *GithubController) ListPatchesInPullRequest(e echo.Context) error {
	cc := e.(*web.AppContext)

	repoName := e.Param("repo")
	prNumber64, err := strconv.ParseInt(e.Param("pull_number"), 10, 64)
	if err != nil {
		cc.AppLogger.Error("Failed to parse PR number", zap.Error(err))
		return err
	}
	prNumber := int(prNumber64)
	owner := e.Param("owner")

	patches, err := controller.listAllContentsInPullRequest(e, repoName, prNumber, owner)
	if err != nil {
		cc.AppLogger.Error("Failed to list pull request files", zap.Error(err))
		return err
	}

	return e.JSON(http.StatusOK, patches)
}

func (controller *GithubController) GenerateCodeReview(e echo.Context) error {
	cc := e.(*web.AppContext)

	repoName := e.Param("repo")
	prNumber64, err := strconv.ParseInt(e.Param("pull_number"), 10, 64)
	if err != nil {
		cc.AppLogger.Error("Failed to parse PR number", zap.Error(err))
		return err
	}
	prNumber := int(prNumber64)
	owner := e.Param("owner")

	patches, err := controller.listAllContentsInPullRequest(e, repoName, prNumber, owner)
	if err != nil {
		cc.AppLogger.Error("Failed to list pull request files", zap.Error(err))
		return err
	}

	existingComments, _, err := controller.GithubClient.PullRequests.ListComments(
		controller.Context,
		owner,
		repoName,
		prNumber,
		nil,
	)

	if err != nil {
		cc.AppLogger.Error("Failed to list pull request comments", zap.Error(err))
		return err
	}

	for _, existingComment := range existingComments {
		controller.GithubClient.PullRequests.DeleteComment(controller.Context, owner, repoName, existingComment.GetID())
	}

	for _, patch := range patches {
		reviewSuggestionResponse, err := controller.requestCodeReview(e, patch.StartLine, patch.EndLine, patch.Content)
		if err != nil {
			cc.AppLogger.Error("Failed to request code review", zap.Error(err))
			return err
		}

		for _, suggestion := range reviewSuggestionResponse.ReviewSuggestions {
			err = controller.postCommentToPR(e, repoName, prNumber, owner, patch.Filename, suggestion.Position, suggestion.Comment)
			if err != nil {
				cc.AppLogger.Error("Failed to post comment on GitHub", zap.Error(err))
				return err
			}
			fmt.Println("suggestion", suggestion)
		}

	}

	return e.JSON(http.StatusOK, patches)
}

func (controller *GithubController) listAllContentsInPullRequest(e echo.Context, repoName string, prNumber int, owner string) ([]domain.FileContentResponse, error) {
	cc := e.(*web.AppContext)

	// Get the repository and PR information
	fileContents := []domain.FileContentResponse{}

	// Get files in the pull request
	prFiles, _, err := controller.GithubClient.PullRequests.ListFiles(
		controller.Context,
		owner,
		repoName,
		prNumber,
		nil,
	)
	if err != nil {
		cc.AppLogger.Error("Failed to list pull request files", zap.Error(err))
		return nil, err
	}

	for _, file := range prFiles {
		changedBatches, err := ParsePatch(file.GetPatch())
		if err != nil {
			cc.AppLogger.Error("Failed to parse patch", zap.Error(err))
			continue
		}

		for _, batch := range changedBatches {
			fileContents = append(fileContents, domain.FileContentResponse{
				Filename:  file.GetFilename(),
				Content:   batch.Content,
				StartLine: batch.StartLine,
				EndLine:   batch.EndLine,
				Position:  batch.Position,
			})
		}
	}

	return fileContents, nil

}

type ChangeBatch struct {
	StartLine int    // Starting line number of the batch
	EndLine   int    // Ending line number of the batch
	Content   string // Combined content of the batch for review
	Position  int
}

func ParsePatch(patch string) ([]ChangeBatch, error) {
	var changeBatches []ChangeBatch

	// unified-diff format: @@ -start,count +start,count @@
	// original state represented with -, and the new state is represented with + (does not mean additions and deletions)
	hunkHeaderRegex := regexp.MustCompile(`@@ -\d+(,\d+)? \+(\d+)(,\d+)? @@`)
	lines := strings.Split(patch, "\n")
	var currentPosition int
	var currentLine int
	var currentBatch *ChangeBatch

	for _, line := range lines {
		if matches := hunkHeaderRegex.FindStringSubmatch(line); matches != nil {
			// Parse starting line number
			startingLine := matches[2]
			currentLine, _ = strconv.Atoi(startingLine)

			// Reset position for the new hunk
			currentPosition = 0

			// Finalize ongoing batch
			if currentBatch != nil {
				changeBatches = append(changeBatches, *currentBatch)
				currentBatch = nil
			}
		} else if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			// Increment position for added lines
			currentPosition++
			if currentBatch == nil {
				currentBatch = &ChangeBatch{
					StartLine: currentLine,
					Content:   line[1:], // Remove "+"
					Position:  currentPosition,
				}
			} else {
				currentBatch.Content += "\n" + line[1:]
			}
			currentBatch.EndLine = currentLine
			currentLine++

		} else if strings.HasPrefix(line, " ") {
			// Increment line for context lines
			currentLine++
		} else if currentBatch != nil {
			// Finalize batch for unrelated lines
			changeBatches = append(changeBatches, *currentBatch)
			currentBatch = nil
		}
	}

	// Finalize the last batch
	if currentBatch != nil {
		changeBatches = append(changeBatches, *currentBatch)
	}

	return changeBatches, nil
}

func (controller *GithubController) requestCodeReview(
	e echo.Context,
	startLine int,
	endLine int,
	content string,
) (domain.ReviewSuggestionsResponse, error) {
	cc := e.(*web.AppContext)

	schemaParam := openai.ResponseFormatJSONSchemaJSONSchemaParam{
		Name:        openai.F("review_suggestion_response"),
		Description: openai.F("A list of code review suggestions"),
		Schema:      openai.F(domain.ReviewSuggestionSchema),
		Strict:      openai.Bool(true),
	}

	chatCompletion, err := controller.OpenAIClient.Chat.Completions.New(context.Background(), openai.ChatCompletionNewParams{
		Messages: openai.F([]openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(`You are developing an automated code review tool for the Engineering department of a technology/software company. Given a code snippet or file, analyze the code's quality and provide suggestions for improvement. Identify common issues such as code smells, anti-patterns, potential bugs, performance bottlenecks, and security vulnerabilities. Offer actionable recommendations to address these issues and improve the overall quality of the code.


			**Avoid commenting on**:
			- Code formatting, style, or linting issues (e.g., missing semicolons, indentation, naming conventions).
			- Documentation or code comments, unless they are completely incorrect or misleading.
			- Any automatically generated code, including files like package.json, go.mod, or auto-generated code/comments.
			- Trivial or subjective issues (e.g., preferences for variable names, small logical redundancies).
						
			**Important**:
			- Only provide feedback when there is a clear, significant issue or improvement to suggest.
			- Do not return anything if there are no critical comments."`),
			openai.UserMessage(fmt.Sprintf(
				"Here is the code to review (lines %d to %d):\n\n%s",
				startLine, endLine, content,
			)),
		}),
		ResponseFormat: openai.F[openai.ChatCompletionNewParamsResponseFormatUnion](
			openai.ResponseFormatJSONSchemaParam{
				Type:       openai.F(openai.ResponseFormatJSONSchemaTypeJSONSchema),
				JSONSchema: openai.F(schemaParam),
			},
		),
		Model: openai.F(openai.ChatModelGPT4oMini),
	})

	if err != nil {
		cc.AppLogger.Error("Failed to get response from OpenAI", zap.Error(err))
		return domain.ReviewSuggestionsResponse{}, err
	}

	// The model responds with a JSON string, so parse it into a struct
	reviewSuggestions := domain.ReviewSuggestionsResponse{}
	err = json.Unmarshal([]byte(chatCompletion.Choices[0].Message.Content), &reviewSuggestions)
	if err != nil {
		cc.AppLogger.Error("Failed to parse response from OpenAI", zap.Error(err))
		return domain.ReviewSuggestionsResponse{}, err
	}

	return reviewSuggestions, nil
}

func (controller *GithubController) postCommentToPR(
	e echo.Context,
	repoName string,
	prNumber int,
	owner string,
	file string,
	position int,
	comment string,
) error {
	cc := e.(*web.AppContext)

	pr, _, err := controller.GithubClient.PullRequests.Get(
		controller.Context,
		owner,
		repoName,
		prNumber,
	)
	if err != nil {
		cc.AppLogger.Error("Failed to get pull request details", zap.Error(err))
		return err
	}

	commitID := pr.GetHead().GetSHA()

	prComment := &github.PullRequestComment{
		Body:     github.Ptr(comment),
		Path:     github.Ptr(file),
		Position: github.Ptr(position),
		CommitID: github.Ptr(commitID),
	}

	_, _, err = controller.GithubClient.PullRequests.CreateComment(
		controller.Context,
		owner,
		repoName,
		prNumber,
		prComment,
	)

	if err != nil {
		cc.AppLogger.Error("Failed to post comment on GitHub", zap.Error(err))
	}

	return err
}
