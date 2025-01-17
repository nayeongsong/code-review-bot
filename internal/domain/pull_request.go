package domain

type PullRequestEvent struct {
	Action      string      `json:"action"`
	PullRequest PullRequest `json:"pull_request"`
	Repository  Repository  `json:"repository"`
}

type GitHubUser struct {
	Login string `json:"login"`
	Id    int    `json:"id"`
	Url   string `json:"url"`
}

type PullRequest struct {
	Id      int        `json:"id"`
	Number  int        `json:"number"`
	Title   string     `json:"title"`
	Body    string     `json:"body"`
	User    GitHubUser `json:"user"`
	HTMLUrl string     `json:"html_url"`
	DiffUrl string     `json:"diff_url"`
}

type Repository struct {
	Id       int        `json:"id"`
	Name     string     `json:"name"`
	FullName string     `json:"full_name"`
	Owner    GitHubUser `json:"owner"`
}
