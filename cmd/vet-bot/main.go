package main

import (
	"context"
	"log"
	"os"
	"sync"
	"time"

	"github.com/google/go-github/v32/github"
	"github.com/kalexmills/github-vet/internal/ratelimit"
	"golang.org/x/oauth2"
)

const findingsOwner = "github-vet"
const findingsRepo = "rangeclosure-findings"

func main() {
	logFilename := time.Now().Format("01-02-2006") + ".log"
	logFile, err := os.OpenFile(logFilename, os.O_APPEND|os.O_CREATE, 0666)
	defer logFile.Close()
	if err != nil {
		log.Fatalf("cannot open log file for writing: %v", err)
	}
	log.SetOutput(logFile)

	sampler, err := NewRepositorySampler("repos.csv", "visited.csv")
	defer sampler.Close()
	if err != nil {
		log.Fatalf("can't start sampler: %v", err)
	}

	ghToken, ok := os.LookupEnv("GITHUB_TOKEN")
	if !ok {
		log.Fatalln("could not find GITHUB_TOKEN environment variable")
	}
	vetBot := NewVetBot(ghToken)
	issueReporter, err := NewIssueReporter(&vetBot, "issues.csv")
	if err != nil {
		log.Fatalf("can't start issue reporter: %v", err)
	}
	for {
		err := sampler.Sample(func(r Repository) error {
			return VetRepositoryBulk(&vetBot, issueReporter, r)
		})
		if err != nil {
			log.Printf("stopping scan due to error :%v", err)
			break
		}
	}

	vetBot.wg.Wait()
}

// VetBot wraps the GitHub client and context used for all GitHub API requests.
type VetBot struct {
	client     *ratelimit.Client
	reportFunc func(bot *VetBot, result VetResult)
	wg         sync.WaitGroup
}

// NewVetBot creates a new bot using the provided GitHub token for access.
func NewVetBot(token string) VetBot {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: string(token)},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)
	limited, err := ratelimit.NewClient(ctx, client)
	if err != nil {
		panic(err)
	}
	return VetBot{
		client: &limited,
	}
}
