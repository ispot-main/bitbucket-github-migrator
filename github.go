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

func createRepo(gh *github.Client, repo *bitbucket.Repository, config settings) *github.Repository {
	var visibility string
	if repo.Is_private {
		visibility = config.visibility
	} else {
		visibility = "public"
	}
	ghRepo := &github.Repository{
		Name:          github.Ptr(repo.Slug),
		Visibility:    github.Ptr(visibility),
		Description:   github.Ptr(repo.Description),
		DefaultBranch: github.Ptr(repo.Mainbranch.Name),
		Language:      github.Ptr(repo.Language),
		Organization: &github.Organization{
			Name: github.Ptr(config.ghOrg),
		},
		Topics: []string{"migratedFromBitbucket", cleanTopic(repo.Project.Name)},
	}

	if config.dryRun {
		return ghRepo
	}

	fmt.Printf("Creating repo %s/%s\n", config.ghOrg, repo.Slug)
	repoCreated := false
	_, _, err := gh.Repositories.Create(context.Background(), config.ghOrg, ghRepo)
	if err != nil {
		if strings.Contains(err.Error(), "name already exists on this account") {
			if !config.overwrite {
				log.Fatalf("Refusing to overwrite Github repo %s", repo.Slug)
			}
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
		response, _, _ := gh.Repositories.Get(context.Background(), config.ghOrg, repo.Slug)
		if response != nil {
			fmt.Println("Repo has been created!")
			return ghRepo
		}
		fmt.Printf("Waiting for repo %s to be available on GitHub (attempt %d)...", repo.Slug, i+1)
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

func updateCustomProperties(gh *github.Client, githubOrg string, ghRepo *github.Repository, dryRun bool, projectName string) {
	customProps := []*github.CustomPropertyValue{
		{
			PropertyName: "bitbucket",
			Value:        "true",
		},
		{
			PropertyName: "project",
			Value:        cleanTopic(projectName),
		},
	}
	if dryRun {
		return
	}
	gh.Repositories.CreateOrUpdateCustomProperties(context.Background(), githubOrg, *ghRepo.Name, customProps)
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

// cleans pr summary to nicely display in Github
func cleanBitbucketPRSummary(prSummary string) string {
	prSummary = strings.ReplaceAll(prSummary, "{: data-inline-card='' }", "")
	prSummary = strings.ReplaceAll(prSummary, "\u200c", "") // weird non-printing char, ignore
	return prSummary
}

// migrate open pull requests
func migrateOpenPrs(gh *github.Client, githubOrg string, ghRepo *github.Repository, prs *PullRequests, dryRun bool) {
	for _, pr := range prs.Values {
		if pr.State != "OPEN" {
			continue
		}
		prSummary := cleanBitbucketPRSummary(pr.Summary.Raw)
		text := fmt.Sprintf("PR originally created by %s on %s. Migrated from bitbucket on %s\n\n---\n%s", pr.Author["display_name"].(string), pr.CreatedOn, time.Now().Format(time.RFC3339Nano), prSummary)
		title := "Historical Bitbucket PR #" + strconv.Itoa(pr.ID) + ": " + pr.Title
		branch := pr.Source["branch"].(map[string]any)["name"].(string)
		gh_pr := &github.NewPullRequest{
			Title: &title,
			Body:  &text,
			Head:  &branch,
			Base:  ghRepo.DefaultBranch,
			Draft: &pr.Draft,
		}
		if dryRun {
			return
		}
		newPr, _, err := gh.PullRequests.Create(context.Background(), githubOrg, *ghRepo.Name, gh_pr)
		if err != nil {
			if strings.Contains(err.Error(), "A pull request already exists") {
				fmt.Printf("Skipping PR creation for PR %s, PR already exists\n", strconv.Itoa(pr.ID))
				// sleep for .5s to help avoid github rate limit
				time.Sleep(time.Millisecond * 500)
				continue
			} else {
				log.Fatalf("failed to create PR %s, error: %s", strconv.Itoa(pr.ID), err)
			}
		}
		fmt.Printf("Migrated BB PR %s as GH PR %s\n", strconv.Itoa(pr.ID), strconv.Itoa(*newPr.Number))

		// sleep for .5s to help avoid github rate limit
		time.Sleep(time.Millisecond * 500)
	}
}

// create pull requests
func createClosedPrs(gh *github.Client, githubOrg string, ghRepo *github.Repository, prs *PullRequests, dryRun bool) {
	for _, pr := range prs.Values {
		if pr.State != "MERGED" {
			continue
		}

		prSummary := cleanBitbucketPRSummary(pr.Summary.Raw)
		text := fmt.Sprintf("**Bitbucket PR created on %s by %s**\n\n---\n%s", pr.CreatedOn, pr.Author["display_name"].(string), prSummary)
		title := "Historical Bitbucket PR #" + strconv.Itoa(pr.ID) + ": " + pr.Title
		issue := &github.IssueRequest{
			Title:  &title,
			Body:   &text,
			Labels: &[]string{"bitbucketPR"},
			State:  github.Ptr("closed"),
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

		// we can't create a closed issue directly so we have to edit the issue to close it
		_, _, err = gh.Issues.Edit(context.Background(), githubOrg, *ghRepo.Name, *issueResponse.Number, issue)
		if err != nil {
			log.Fatalf("failed to close issue %s: %s", *issueResponse.URL, err)
		}
		// sleep for .5s to help avoid github rate limit
		time.Sleep(time.Millisecond * 500)
	}
}

func runProgram(repoFolder string, program string) ([]byte, error) {
	if program != "noop" {
		cmd := exec.Command(program, repoFolder)
		return cmd.CombinedOutput()
	} else {
		return []byte{}, nil
	}
}

// pushes all repo branches&tags to Github with --mirror option.
// default branch may get updated as a side-effect
func pushRepoToGithub(repoFolder string, repoName string, config settings) {
	const newOrigin string = "newOrigin"

	cmd := exec.Command("git", "remote", "add", newOrigin, fmt.Sprintf("https://github.com/%s/%s.git", config.ghOrg, repoName))
	cmd.Dir = repoFolder
	output, err := cmd.CombinedOutput()
	fmt.Print(string(output))
	if err != nil {
		log.Fatalf("Failed to add new git origin: %s\nOutput: %s", err, string(output))
	}

	output, err = runProgram(repoFolder, config.runProgram)
	fmt.Print(string(output))
	if err != nil {
		log.Fatalf("Failed to run custom program %s. err: %s", config.runProgram, err)
	}

	if config.dryRun {
		return
	}

	fmt.Println("Pushing repo", repoName, "to github")

	cmd = exec.Command("git", "push", newOrigin, "--mirror")
	cmd.Dir = repoFolder
	output, err = cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Failed to push: %s\nOutput: %s", err, string(output))
	}
	fmt.Print(string(output))
}
