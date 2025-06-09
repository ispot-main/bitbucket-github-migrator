package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/google/go-github/v72/github"
	"github.com/joho/godotenv"
	"github.com/ktrysmt/go-bitbucket"
)

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	bbWorkspace := os.Getenv("BITBUCKET_WORKSPACE")
	bbUsername := os.Getenv("BITBUCKET_USER")
	bbPassword := os.Getenv("BITBUCKET_TOKEN")
	ghOrg := os.Getenv("GITHUB_ORG")
	ghToken := os.Getenv("GITHUB_TOKEN")
	dryRun := getEnvVarAsBool("GITHUB_DRYRUN")
	overwrite := getEnvVarAsBool("GITHUB_OVERWRITE")
	repoFile := os.Getenv("REPO_FILE")

	if bbWorkspace == "" || bbUsername == "" || bbPassword == "" {
		fmt.Println("BITBUCKET_WORKSPACE or BITBUCKET_USER or BITBUCKET_TOKEN not set in .env file or env vars")
		os.Exit(2)
	}

	if ghOrg == "" || ghToken == "" {
		fmt.Println("GITHUB_ORG or GITHUB_TOKEN not set in .env file or env vars")
		os.Exit(2)
	}

	repos := parseRepos(repoFile)

	bitbucketClient := bitbucket.NewBasicAuth(bbUsername, bbPassword)
	githubClient := github.NewClient(nil).WithAuthToken(ghToken)

	migrateRepos(githubClient, bitbucketClient, bbWorkspace, ghOrg, repos, dryRun, overwrite)
}

func getEnvVarAsBool(envVar string) bool {
	result, err := strconv.ParseBool(os.Getenv(envVar))
	if err != nil {
		fmt.Println("could not parse bool env var ", envVar)
		os.Exit(2)
	}
	return result
}

func parseRepos(repoFile string) []string {
	var repos []string
	if repoFile == "" {
		fmt.Println("You must supply a list of names of repos to migrate in REPO_FILE")
		os.Exit(2)
	}
	data, err := os.ReadFile(strings.TrimSpace(repoFile))
	if err != nil {
		log.Fatalf("could not read file %s", repoFile)
	}
	repos = strings.Split(string(data), "\n")

	cleaned_repos := []string{}
	for _, repo := range repos {
		repo = strings.TrimSpace(repo)
		if repo != "" {
			// ignore commented out repos
			if repo[0] == "#"[0] {
				continue
			}
			// bitbucket replaces invalid chars with -
			// see https://support.atlassian.com/bitbucket-cloud/kb/what-is-a-repository-slug/
			repo = strings.ReplaceAll(repo, " ", "-")
			repo = strings.ReplaceAll(repo, "/", "-")
			repo = strings.ReplaceAll(repo, "+", "-")
			repo = strings.ReplaceAll(repo, "&", "-")
			repo = strings.ReplaceAll(repo, "(", "-")
			repo = strings.ReplaceAll(repo, ")", "-")
			cleaned_repos = append(cleaned_repos, repo)
		}
	}
	return cleaned_repos
}

func migrateRepos(gh *github.Client, bb *bitbucket.Client, bbWorkspace string, ghOrg string, repoList []string, dryRun bool, overwrite bool) {
	if dryRun {
		fmt.Println("Dry Run - not actually migrating anything")
	}

	for _, repo := range repoList {
		migrateRepo(gh, bb, bbWorkspace, ghOrg, repo, dryRun, overwrite)
	}
}

func migrateRepo(gh *github.Client, bb *bitbucket.Client, bbWorkspace string, ghOrg string, repoName string, dryRun bool, overwrite bool) {
	fmt.Println("Getting bitbucket settings & downloading ", repoName)
	bbRepo := getRepo(bb, bbWorkspace, repoName)
	repoFolder := cloneRepo(bbWorkspace, repoName)
	prs := getPrs(bb, bbWorkspace, repoName, bbRepo.Mainbranch.Name)

	fmt.Println("Migrating to Github")
	ghRepo := createRepo(gh, ghOrg, bbRepo, dryRun, overwrite)
	pushRepoToGithub(ghOrg, repoFolder, *ghRepo.Name, dryRun)
	// defaultBranch gets overwritten when we git push for some reason
	// we call updateRepo to switch it back
	// Also useful if repo is already created in Github and we want to update with latest repo settings from bitbucket
	updateRepo(gh, ghOrg, ghRepo, dryRun)
	updateRepoTopics(gh, ghOrg, ghRepo, dryRun)
	createPrs(gh, ghOrg, ghRepo, prs, dryRun)
	fmt.Println("done migrating repo")
}
