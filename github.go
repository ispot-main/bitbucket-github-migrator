package main

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v72/github"
	"github.com/ktrysmt/go-bitbucket"
)

// replaces invalid chars in input that are not allowed in Github topics
func cleanTopic(input string) string {
	return strings.ReplaceAll(strings.ToLower(input), " ", "-")
}

func createRepo(gh *github.Client, githubOrg string, repo *bitbucket.Repository, dryRun bool) *github.Repository {
	ghRepo := &github.Repository{
		Name:          github.Ptr(repo.Slug),
		Private:       github.Ptr(repo.Is_private),
		Description:   github.Ptr(repo.Description),
		DefaultBranch: github.Ptr(repo.Mainbranch.Name),
		Language:      github.Ptr(repo.Language),
		Organization: &github.Organization{
			Name: github.Ptr(githubOrg),
		},
		Topics: []string{"migratedFromBitbucket", cleanTopic(repo.Project.Name)},
	}

	if dryRun {
		return ghRepo
	}

	// todo bitbucket project as custom property?
	fmt.Printf("Creating repo %s/%s", githubOrg, repo.Slug)
	repoCreated := false
	_, _, err := gh.Repositories.Create(context.Background(), githubOrg, ghRepo)
	if err != nil {
		if strings.Contains(err.Error(), "name already exists on this account") {
			// it's fine if a repo already exists
			// if it's not just a earlier version of the repo we are migrating the git push will fail
			repoCreated = true
		} else {
			log.Fatalf("failed to create repo %s, error: %s", repo.Slug, err)
		}
	}

	if repoCreated {
		return ghRepo
	}

	// The repository might not have been created yet
	// Wait for the repository to be available
	for i := 0; i < 20; i++ {
		time.Sleep(200 * time.Millisecond)
		response, _, _ := gh.Repositories.Get(context.Background(), githubOrg, repo.Slug)
		if response != nil {
			log.Print("Repo has been created!")
			return ghRepo
		}
		log.Printf("Waiting for repo %s to be available on GitHub (attempt %d)...", repo.Slug, i+1)
		// Wait for a short period before retrying
		time.Sleep(1 * time.Second)
	}
	log.Fatalf("Repo has still not been created")
	return nil
}

// you need to call this after createRepo and pushRepoToGithub because
// topics can't be updated until the repository has contents
func updateRepoTopics(gh *github.Client, githubOrg string, ghRepo *github.Repository, dryRun bool) {
	if dryRun {
		fmt.Println("Mock updating repo topics")
		return
	}
	fmt.Printf("Updating repo %s/%s topics\n", githubOrg, *ghRepo.Name)
	_, _, err := gh.Repositories.ReplaceAllTopics(context.Background(), githubOrg, *ghRepo.Name, ghRepo.Topics)
	if err != nil {
		log.Fatalf("failed to update topics for repo %s, error: %s", *ghRepo.Name, err)
	}
}

func updateRepo(gh *github.Client, githubOrg string, ghRepo *github.Repository, dryRun bool) {
	if dryRun {
		fmt.Println("Mock updating repo default branch")
		return
	}
	fmt.Printf("Updating repo %s/%s default branch\n", githubOrg, *ghRepo.Name)
	_, _, err := gh.Repositories.Edit(context.Background(), githubOrg, *ghRepo.Name, ghRepo)
	if err != nil {
		log.Fatalf("failed to update repo %s, error: %s", *ghRepo.Name, err)
	}
}

// create pull requests
func createPrs(gh *github.Client, githubOrg string, ghRepo *github.Repository, prs *PullRequests, dryRun bool) {
	pr := prs.Values[0]
	text := fmt.Sprintf("**Bitbucket PR created on %s by %s**\n\n%s", pr.CreatedOn, pr.Author["display_name"].(string), pr.Summary.Raw)
	title := "Bitbucket PR #" + strconv.Itoa(pr.ID) + ": " + pr.Title
	issue := &github.IssueRequest{
		Title: &title,
		Body:  &text,
	}
	if dryRun {
		return
	}
	fmt.Printf("Updating issue for PR %s\n", strconv.Itoa(pr.ID))
	issueResponse, _, err := gh.Issues.Create(context.Background(), githubOrg, *ghRepo.Name, issue)
	if err != nil {
		log.Fatalf("failed to create issue for PR %s, error: %s", strconv.Itoa(pr.ID), err)
	}

	commitHash := pr.MergeCommit.Hash
	comment := &github.RepositoryComment{
		Body: github.Ptr("Bitbucket PR details: #" + strconv.Itoa(*issueResponse.Number)),
	}
	_, _, err = gh.Repositories.CreateComment(context.Background(), githubOrg, *ghRepo.Name, commitHash, comment)
	if err != nil {
		log.Fatalf("failed to comment on commit %s: %s", commitHash, err)
	}
}

// pushes all repo branches&tags to Github with --mirror option.
// default branch may get updated as a side-effect
func pushRepoToGithub(githubOrg string, repoFolder string, repoName string, dryRun bool) {
	const newOrigin string = "newOrigin"

	cmd := exec.Command("git", "remote", "add", newOrigin, fmt.Sprintf("https://github.com/%s/%s.git", githubOrg, repoName))
	cmd.Dir = repoFolder
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Failed to add new git origin: %s\nOutput: %s", err, string(output))
	}
	fmt.Println(string(output))

	if dryRun {
		return
	}

	log.Println("Pushing repo", repoName, "to github")

	cmd = exec.Command("git", "push", newOrigin, "--mirror")
	cmd.Dir = repoFolder
	output, err = cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Failed to push: %s\nOutput: %s", err, string(output))
	}
	fmt.Println(string(output))
}
