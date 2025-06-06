package main

import (
	"cmp"
	"errors"
	"fmt"
	"lo feg"
	"os"
	"os/exec"
	"slices"

	"github.com/ktrysmt/go-bitbucket"
	"github.com/mitchellh/mapstructure"
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

func getPrs(bb *bitbucket.Client, owner string, repo string, destinationBranch string) *PullRequests {
	opt := &bitbucket.PullRequestsOptions{
		Owner:             owner,
		RepoSlug:          repo,
		DestinationBranch: destinationBranch,
		States:            []string{"MERGED"},
	}
	fmt.Println("getting prs for ", repo)
	response, err := bb.Repositories.PullRequests.Gets(opt)
	if err != nil {
		panic(err)
	}
	prs, err := decodePullRequests(response)
	if err != nil {
		panic(fmt.Sprintf("error decoding PRs: %s", err))
	}
	slices.SortFunc(prs.Values, func(i PullRequest, j PullRequest) int {
		return cmp.Compare(i.ID, j.ID)
	})
	return prs
}

/////////////////////////////////

var stringToTimeHookFunc = mapstructure.StringToTimeHookFunc("2006-01-02T15:04:05.000000+00:00")

func DecodeError(e map[string]interface{}) error {
	var bitbucketError bitbucket.BitbucketError
	err := mapstructure.Decode(e["error"], &bitbucketError)
	if err != nil {
		return err
	}

	return errors.New(bitbucketError.Message)
}

type PullRequests struct {
	Size     int
	Page     int
	Pagelen  int
	Next     string
	Previous string
	Values   []PullRequest
}

type PullRequest struct {
	c *bitbucket.Client

	Type              string
	ID                int
	Title             string
	Rendered          PRRendered
	Summary           PRSummary
	State             string
	Author            map[string]any
	Source            map[string]any
	Destination       map[string]any
	MergeCommit       PRMergeCommit  `mapstructure:"merge_commit"`
	CommentCount      int            `mapstructure:"comment_count"`
	TaskCount         int            `mapstructure:"task_count"`
	CloseSourceBranch bool           `mapstructure:"close_source_branch"`
	ClosedBy          map[string]any `mapstructure:"closed_by"`
	Reason            string
	CreatedOn         string `mapstructure:"created_on"`
	UpdatedOn         string `mapstructure:"updated_on"`
	Reviewers         []map[string]any
	Participants      []map[string]any
	Draft             bool
	Queued            bool
}

type PRRendered struct {
	Title       PRText
	Description PRText
	Reason      PRText
}

type PRText struct {
	Raw    string
	Markup string
	HTML   string
}

type PRSummary struct {
	Raw    string
	Markup string
	HTML   string
}

type PRMergeCommit struct {
	Hash string
}

func decodePullRequests(reposResponse interface{}) (*PullRequests, error) {
	prResponseMap, ok := reposResponse.(map[string]interface{})
	if !ok {
		return nil, errors.New("Not a valid format")
	}

	repoArray := prResponseMap["values"].([]interface{})
	var prs []PullRequest
	for _, repoEntry := range repoArray {
		repo, err := decodePullRequest(repoEntry)
		if err == nil {
			prs = append(prs, *repo)
		} else {
			return nil, err
		}
	}

	page, ok := prResponseMap["page"].(float64)
	if !ok {
		page = 0
	}

	pagelen, ok := prResponseMap["pagelen"].(float64)
	if !ok {
		pagelen = 0
	}
	size, ok := prResponseMap["size"].(float64)
	if !ok {
		size = 0
	}

	pullRequests := PullRequests{
		Page:    int(page),
		Pagelen: int(pagelen),
		Size:    int(size),
		Values:  prs,
	}
	return &pullRequests, nil
}

func decodePullRequest(response interface{}) (*PullRequest, error) {
	repoMap := response.(map[string]interface{})

	if repoMap["type"] == "error" {
		return nil, DecodeError(repoMap)
	}

	var pr = new(PullRequest)
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Metadata:   nil,
		Result:     pr,
		DecodeHook: stringToTimeHookFunc,
	})
	if err != nil {
		return nil, err
	}
	err = decoder.Decode(repoMap)
	if err != nil {
		return nil, err
	}

	return pr, nil
}
