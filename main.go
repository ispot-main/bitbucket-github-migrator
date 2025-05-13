package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"

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

func getRepo(bb *bitbucket.Client, owner string, repoName string) *bitbucket.Repository {
	ro := &bitbucket.RepositoryOptions{
		Owner:    owner,
		RepoSlug: repoName,
	}
	repo, err := bb.Repositories.Repository.Get(ro)
	if err != nil {
		panic(err)
	}
	return repo
}

// clones repo to a temp folder
func cloneRepo(owner string, repo string) (tempfolderpath string) {
	tempDir, err := os.MkdirTemp("", fmt.Sprintf("%s-%s-*", owner, repo))
	if err != nil {
		log.Fatalf("Failed to create temp directory: %s", err)
	}

	cloneURL := fmt.Sprintf("https://bitbucket.org/%s/%s.git", owner, repo)
	fmt.Printf("Cloning repository %s to %s\n", repo, tempDir)

	cmd := exec.Command("git", "clone", cloneURL, tempDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Failed to clone repository: %s\nOutput: %s", err, string(output))
	}
	fmt.Println(string(output))

	return tempDir
}
