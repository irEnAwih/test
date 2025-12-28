package config

import (
	"flag"
	"github.com/joho/godotenv"
	"go-pr-review/internal/domain"
	"log"
	"os"
)

func Load() *domain.Config {
	_ = godotenv.Load()
	cfg := &domain.Config{
		GithubToken: os.Getenv("GITHUB_TOKEN"),
		OpenAIKey:   os.Getenv("OPENAI_API_KEY"),
	}

	owner := flag.String("owner", "irEnAwih", "Repo Owner")
	repo := flag.String("repo", "test", "Repo Name")
	pr := flag.Int("pr", 1, "PR Number")
	flag.Parse()

	cfg.RepoOwner = *owner
	cfg.RepoName = *repo
	cfg.PRNumber = *pr

	if cfg.GithubToken == "" || cfg.OpenAIKey == "" {
		log.Fatal("Missing env vars: GITHUB_TOKEN or OPENAI_API_KEY")
	}
	if cfg.RepoOwner == "" || cfg.RepoName == "" || cfg.PRNumber == 0 {
		log.Fatal("Usage: app -owner=... -repo=... -pr=...")
	}

	return cfg
}
