package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/google/go-github/v72/github"
	"github.com/joho/godotenv"
	"github.com/ktrysmt/go-bitbucket"
)

func main() {
	var repoListFile string
	var dryRun bool
	flag.StringVar(&repoListFile, "file", "", "List of repositories to migrate with one repo url on each line")
	flag.BoolVar(&dryRun, "dryRun", false, "Whether to do a dry run or not")
	// todo convert repolistFile to repoList
	flag.Parse()
	if dryRun {
		fmt.Println("Dry Run - not actually migrating anything")
		// todo rest of dryrun logic
	}

	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	bbOrg := os.Getenv("BITBUCKET_ORG")
	bbUsername := os.Getenv("BITBUCKET_USER")
	bbPassword := os.Getenv("BITBUCKET_TOKEN")
	ghOrg := os.Getenv("GITHUB_ORG")
	ghToken := os.Getenv("GITHUB_TOKEN")

	if bbOrg == "" || bbUsername == "" || bbPassword == "" {
		fmt.Println("BITBUCKET_ORG or BITBUCKET_USER or BITBUCKET_TOKEN not set in .env file or env vars")
		os.Exit(2)
	}

	if ghOrg == "" || ghToken == "" {
		fmt.Println("GITHUB_ORG or GITHUB_TOKEN not set in .env file or env vars")
		os.Exit(2)
	}

	bitbucketClient := bitbucket.NewBasicAuth(bbUsername, bbPassword)
	githubClient := github.NewClient(nil).WithAuthToken(ghToken)

	migrateRepos(githubClient, bitbucketClient, bbOrg, ghOrg, []string{"atc-cli"}, dryRun)
}

func migrateRepos(gh *github.Client, bb *bitbucket.Client, bbOrg string, ghOrg string, repoList []string, dryRun bool) {
	for _, repo := range repoList {
		migrateRepo(gh, bb, bbOrg, ghOrg, repo)
	}
}

func migrateRepo(gh *github.Client, bb *bitbucket.Client, bbOrg string, ghOrg string, repoName string) {
	bbRepo := getRepo(bb, bbOrg, repoName)
	repoFolder := cloneRepo(bbOrg, repoName)
	ghRepo := createRepo(gh, ghOrg, bbRepo)
	pushRepoToGithub(ghOrg, repoFolder, *ghRepo.Name)
	updateRepoTopics(gh, ghOrg, ghRepo)
	//prs := getPrs(bb, owner, repoName, repo.Mainbranch.Name)
	//fmt.Println("mock migrating prs: ", prs.Values[0].Title)
	fmt.Println("done migrating repo")
}
