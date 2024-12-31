package internal

import "github.com/caarlos0/env/v11"

type Configuration struct {
	GithubToken string `env:"GITHUB_TOKEN,notEmpty"`
	OpenAIToken string `env:"OPENAI_API_KEY,notEmpty"`
}

func LoadConfiguration() (Configuration, error) {
	config := Configuration{}
	err := env.Parse(&config)
	if err != nil {
		return config, err
	}
	return config, nil
}
