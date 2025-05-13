package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/google/go-github/v72/github"
	"github.com/joho/godotenv"
	"github.com/ktrysmt/go-bitbucket"
)

func main() {
	var repoListFile string
	var dryRun bool
	flag.StringVar(&repoListFile, "file", "", "List of repositories to migrate with one repo url on each line")
	flag.BoolVar(&dryRun, "dryRun", false, "Whether to do a dry run or not")
	flag.Parse()

	if dryRun {
		fmt.Println("Dry Run - not actually migrating anything")
		// todo rest of dryrun logic
	}

	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	bbWorkspace := os.Getenv("BITBUCKET_WORKSPACE")
	bbUsername := os.Getenv("BITBUCKET_USER")
	bbPassword := os.Getenv("BITBUCKET_TOKEN")
	ghOrg := os.Getenv("GITHUB_ORG")
	ghToken := os.Getenv("GITHUB_TOKEN")
	envVarRepos := os.Getenv("REPOS")

	if bbWorkspace == "" || bbUsername == "" || bbPassword == "" {
		fmt.Println("BITBUCKET_WORKSPACE or BITBUCKET_USER or BITBUCKET_TOKEN not set in .env file or env vars")
		os.Exit(2)
	}

	if ghOrg == "" || ghToken == "" {
		fmt.Println("GITHUB_ORG or GITHUB_TOKEN not set in .env file or env vars")
		os.Exit(2)
	}

	var repos []string
	if envVarRepos != "" {
		repos = strings.Split(string(envVarRepos), ",")
	} else {
		if repoListFile == "" {
			fmt.Println("You must supply a list of repos to migrate")
			os.Exit(2)
		}
		data, err := os.ReadFile(repoListFile)
		if err != nil {
			log.Fatalf("could not read file %s", repoListFile)
		}
		repos = strings.Split(string(data), "\n")
	}

	bitbucketClient := bitbucket.NewBasicAuth(bbUsername, bbPassword)
	githubClient := github.NewClient(nil).WithAuthToken(ghToken)

	migrateRepos(githubClient, bitbucketClient, bbWorkspace, ghOrg, repos, dryRun)
}

func migrateRepos(gh *github.Client, bb *bitbucket.Client, bbWorkspace string, ghOrg string, repoList []string, dryRun bool) {
	for _, repo := range repoList {
		repo = strings.TrimSpace(repo)
		migrateRepo(gh, bb, bbWorkspace, ghOrg, repo)
	}
}

func migrateRepo(gh *github.Client, bb *bitbucket.Client, bbWorkspace string, ghOrg string, repoName string) {
	fmt.Printf("Migrating repo %s\n", repoName)
	bbRepo := getRepo(bb, bbWorkspace, repoName)
	repoFolder := cloneRepo(bbWorkspace, repoName)
	ghRepo := createRepo(gh, ghOrg, bbRepo)
	pushRepoToGithub(ghOrg, repoFolder, *ghRepo.Name)
	updateRepoTopics(gh, ghOrg, ghRepo)
	//prs := getPrs(bb, owner, repoName, repo.Mainbranch.Name)
	//fmt.Println("mock migrating prs: ", prs.Values[0].Title)
	fmt.Println("done migrating repo")
}
