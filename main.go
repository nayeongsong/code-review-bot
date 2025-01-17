package main

import (
	"code-review-bot/internal"
	"code-review-bot/internal/web"
	"code-review-bot/internal/web/routes"
	"context"
	"strings"
	"time"

	"github.com/google/go-github/v68/github"
	"github.com/gregjones/httpcache"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"go.uber.org/zap"
)

func initializeGithubClient(ctx context.Context, githubToken string, logger *zap.Logger) *github.Client {
	client := github.NewClient(httpcache.NewMemoryCacheTransport().Client()).WithAuthToken(githubToken)

	user, _, err := client.Users.Get(ctx, "")
	if err != nil {
		logger.Fatal("Failed to authenticate with GitHub API", zap.Error(err))
	}
	logger.Info("Authenticated with GitHub API", zap.String("username", user.GetLogin()))

	return client
}

func initializeOpenAIClient(openAIToken string, logger *zap.Logger) *openai.Client {
	client := openai.NewClient(option.WithAPIKey(openAIToken))

	logger.Info("OpenAI client initialized")

	return client
}

func main() {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()
	logger.Info("starting server")

	config, err := internal.LoadConfiguration()
	if err != nil {
		logger.Fatal("failed to load configuration", zap.Error(err))
	}

	e := echo.New()
	ctx := context.Background()
	//e.Use(middleware.Recover())
	e.Use(web.CreateAppContext(logger))
	e.Pre(middleware.AddTrailingSlash())
	e.Use(middleware.TimeoutWithConfig(middleware.TimeoutConfig{
		Skipper:                    middleware.DefaultSkipper,
		ErrorMessage:               "request timeout",
		OnTimeoutRouteErrorHandler: func(err error, c echo.Context) {},
		Timeout:                    15 * time.Second,
	}))
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogURI:       true,
		LogStatus:    true,
		LogRemoteIP:  true,
		LogMethod:    true,
		LogRequestID: true,
		LogLatency:   true,
		Skipper: func(c echo.Context) bool {
			return strings.Contains(c.Path(), "public/")
		},
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			logger.Info("request",
				zap.String("method", v.Method),
				zap.String("uri", v.URI),
				zap.String("remoteip", v.RemoteIP),
				zap.String("requestid", v.RequestID),
				zap.Int("status", v.Status),
				zap.Duration("latency", v.Latency),
			)
			return nil
		},
	}))
	e.Use(middleware.RequestID())

	// Initialize the GitHub client
	githubClient := initializeGithubClient(ctx, config.GithubToken, logger)
	openAIClient := initializeOpenAIClient(config.OpenAIToken, logger)

	routes.CreateRoutes(e, githubClient, openAIClient, ctx)
	logger.Info("server started on :8080")
	err = e.Start("0.0.0.0:8080")
	if err != nil {
		logger.Fatal("failed to start server", zap.Error(err))
	}
}
