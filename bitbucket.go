package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/ktrysmt/go-bitbucket"
)

func getRepo(bb *bitbucket.Client, owner string, repoName string) *bitbucket.Repository {
	ro := &bitbucket.RepositoryOptions{
		Owner:    owner,
		RepoSlug: repoName,
	}
	repo, err := bb.Repositories.Repository.Get(ro)
	if err != nil {
		fmt.Printf("Failed to get repo from bitbucket")
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

	cmd := exec.Command("git", "clone", "--mirror", cloneURL, tempDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Failed to clone repository: %s\nOutput: %s", err, string(output))
	}
	fmt.Println(string(output))

	return tempDir
}
