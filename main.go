package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v72/github"
	"github.com/joho/godotenv"
	"github.com/ktrysmt/go-bitbucket"
)

type settings struct {
	bbWorkspace         string
	bbUsername          string
	bbPassword          string
	ghOrg               string
	ghToken             string
	dryRun              bool
	overwrite           bool
	repoFile            string
	migrateRepoContents bool
	migrateRepoSettings bool
	migrateOpenPrs      bool
	migrateClosedPrs    bool
}

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	config := settings{
		bbWorkspace:         os.Getenv("BITBUCKET_WORKSPACE"),
		bbUsername:          os.Getenv("BITBUCKET_USER"),
		bbPassword:          os.Getenv("BITBUCKET_TOKEN"),
		ghOrg:               os.Getenv("GITHUB_ORG"),
		ghToken:             os.Getenv("GITHUB_TOKEN"),
		dryRun:              getEnvVarAsBool("GITHUB_DRYRUN"),
		overwrite:           getEnvVarAsBool("GITHUB_OVERWRITE"),
		repoFile:            os.Getenv("REPO_FILE"),
		migrateRepoContents: getEnvVarAsBool("MIGRATE_REPO_CONTENTS"),
		migrateRepoSettings: getEnvVarAsBool("MIGRATE_REPO_SETTINGS"),
		migrateOpenPrs:      getEnvVarAsBool("MIGRATE_OPEN_PRS"),
		migrateClosedPrs:    getEnvVarAsBool("MIGRATE_CLOSED_PRS"),
	}

	if config.bbWorkspace == "" || config.bbUsername == "" || config.bbPassword == "" {
		fmt.Println("BITBUCKET_WORKSPACE or BITBUCKET_USER or BITBUCKET_TOKEN not set in .env file or env vars")
		os.Exit(2)
	}

	if config.ghOrg == "" || config.ghToken == "" {
		fmt.Println("GITHUB_ORG or GITHUB_TOKEN not set in .env file or env vars")
		os.Exit(2)
	}

	repos := parseRepos(config.repoFile)

	bitbucketClient := bitbucket.NewBasicAuth(config.bbUsername, config.bbPassword)
	githubClient := github.NewClient(nil).WithAuthToken(config.ghToken)

	migrateRepos(githubClient, bitbucketClient, repos, config)
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

func migrateRepos(gh *github.Client, bb *bitbucket.Client, repoList []string, config settings) {
	if config.dryRun {
		fmt.Println("Dry Run - not actually migrating anything")
	}

	for _, repo := range repoList {
		migrateRepo(gh, bb, repo, config)
	}
}

func migrateRepo(gh *github.Client, bb *bitbucket.Client, repoName string, config settings) {
	fmt.Println("Getting bitbucket settings & downloading", repoName)
	bbRepo := getRepo(bb, config.bbWorkspace, repoName)
	var repoFolder string
	if config.migrateRepoContents {
		repoFolder = cloneRepo(config.bbWorkspace, repoName)
	}
	var prs *PullRequests
	if config.migrateOpenPrs || config.migrateClosedPrs {
		prs = getPrs(bb, config.bbWorkspace, repoName, bbRepo.Mainbranch.Name)
	}

	fmt.Println("Migrating to Github")
	ghRepo := createRepo(gh, bbRepo, config)
	if config.migrateRepoContents {
		pushRepoToGithub(config.ghOrg, repoFolder, *ghRepo.Name, config.dryRun)
	} else {
		fmt.Println("Skipping repo contents")
	}
	if config.migrateRepoSettings {
		updateRepo(gh, config.ghOrg, ghRepo, config.dryRun)
		updateRepoTopics(gh, config.ghOrg, ghRepo, config.dryRun)
	} else {
		fmt.Println("Skipping repo settings")
	}
	if config.migrateOpenPrs {
		migrateOpenPrs(gh, config.ghOrg, ghRepo, prs, config.dryRun)
	} else {
		fmt.Println("Skipping open PR's")
	}
	if config.migrateClosedPrs {
		createClosedPrs(gh, config.ghOrg, ghRepo, prs, config.dryRun)
	} else {
		fmt.Println("Skipping closed PR's")
	}
	fmt.Println("done migrating repo")
	fmt.Println()

	// sleep for .5s to help avoid github rate limit
	time.Sleep(time.Millisecond * 500)
}
