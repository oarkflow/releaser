/*
Package git provides git information extraction for Releaser.
*/
package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// Info contains git repository information
type Info struct {
	// CurrentTag is the current tag (if on a tag)
	CurrentTag string

	// PreviousTag is the previous tag
	PreviousTag string

	// Commit is the current commit hash
	Commit string

	// ShortCommit is the short commit hash
	ShortCommit string

	// Branch is the current branch
	Branch string

	// TreeState indicates if the tree is clean or dirty
	TreeState string

	// Summary is the git describe summary
	Summary string

	// TagSubject is the tag annotation subject
	TagSubject string

	// TagBody is the tag annotation body
	TagBody string

	// TagContents is the full tag annotation
	TagContents string

	// CommitDate is the commit date
	CommitDate time.Time

	// CommitTimestamp is the commit timestamp
	CommitTimestamp string

	// URL is the repository URL
	URL string

	// IsGitRepo indicates if this is a git repository
	IsGitRepo bool

	// Prerelease indicates if this is a prerelease
	Prerelease bool

	// Major version
	Major int

	// Minor version
	Minor int

	// Patch version
	Patch int

	// Prerelease suffix
	PrereleaseSuffix string

	// Metadata suffix
	Metadata string
}

// GetInfo extracts git information from the current repository
func GetInfo(ctx context.Context) (*Info, error) {
	info := &Info{}

	// Check if this is a git repository
	if _, err := run("git", "rev-parse", "--git-dir"); err != nil {
		return nil, fmt.Errorf("not a git repository")
	}
	info.IsGitRepo = true

	// Get current commit
	commit, err := run("git", "rev-parse", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("failed to get commit: %w", err)
	}
	info.Commit = strings.TrimSpace(commit)
	info.ShortCommit = info.Commit[:8]

	// Get current branch
	branch, err := run("git", "rev-parse", "--abbrev-ref", "HEAD")
	if err == nil {
		info.Branch = strings.TrimSpace(branch)
	}

	// Get tree state
	status, _ := run("git", "status", "--porcelain")
	if strings.TrimSpace(status) == "" {
		info.TreeState = "clean"
	} else {
		info.TreeState = "dirty"
	}

	// Get commit date
	dateStr, err := run("git", "show", "-s", "--format=%ci", "HEAD")
	if err == nil {
		date, err := time.Parse("2006-01-02 15:04:05 -0700", strings.TrimSpace(dateStr))
		if err == nil {
			info.CommitDate = date
			info.CommitTimestamp = fmt.Sprintf("%d", date.Unix())
		}
	}

	// Get current tag
	tag, err := run("git", "describe", "--tags", "--exact-match", "HEAD")
	if err == nil {
		info.CurrentTag = strings.TrimSpace(tag)
		parseVersion(info, info.CurrentTag)

		// Get tag annotation
		subject, _ := run("git", "tag", "-l", "--format=%(subject)", info.CurrentTag)
		info.TagSubject = strings.TrimSpace(subject)

		body, _ := run("git", "tag", "-l", "--format=%(body)", info.CurrentTag)
		info.TagBody = strings.TrimSpace(body)

		contents, _ := run("git", "tag", "-l", "--format=%(contents)", info.CurrentTag)
		info.TagContents = strings.TrimSpace(contents)
	}

	// Get previous tag
	prevTag, err := run("git", "describe", "--tags", "--abbrev=0", "HEAD^")
	if err == nil {
		info.PreviousTag = strings.TrimSpace(prevTag)
	}

	// Get git describe summary
	summary, err := run("git", "describe", "--tags", "--always", "--dirty")
	if err == nil {
		info.Summary = strings.TrimSpace(summary)
	}

	// Get remote URL
	url, err := run("git", "remote", "get-url", "origin")
	if err == nil {
		info.URL = strings.TrimSpace(url)
	}

	return info, nil
}

// GetCommitsSince returns commits since a given ref
func GetCommitsSince(ctx context.Context, since string) ([]Commit, error) {
	var args []string
	if since != "" {
		args = []string{"log", "--pretty=format:%H|%s|%b|%an|%ae|%ci", since + "..HEAD"}
	} else {
		args = []string{"log", "--pretty=format:%H|%s|%b|%an|%ae|%ci"}
	}

	output, err := run("git", args...)
	if err != nil {
		return nil, err
	}

	var commits []Commit
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 6)
		if len(parts) < 6 {
			continue
		}

		date, _ := time.Parse("2006-01-02 15:04:05 -0700", parts[5])
		commits = append(commits, Commit{
			Hash:        parts[0],
			Subject:     parts[1],
			Body:        parts[2],
			AuthorName:  parts[3],
			AuthorEmail: parts[4],
			Date:        date,
		})
	}

	return commits, nil
}

// Commit represents a git commit
type Commit struct {
	Hash        string
	Subject     string
	Body        string
	AuthorName  string
	AuthorEmail string
	Date        time.Time
}

// FilterCommits filters commits based on patterns
func FilterCommits(commits []Commit, include, exclude []string) []Commit {
	var result []Commit

	for _, c := range commits {
		// Check exclude patterns
		excluded := false
		for _, pattern := range exclude {
			re, err := regexp.Compile(pattern)
			if err != nil {
				continue
			}
			if re.MatchString(c.Subject) {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}

		// Check include patterns (if any)
		if len(include) > 0 {
			included := false
			for _, pattern := range include {
				re, err := regexp.Compile(pattern)
				if err != nil {
					continue
				}
				if re.MatchString(c.Subject) {
					included = true
					break
				}
			}
			if !included {
				continue
			}
		}

		result = append(result, c)
	}

	return result
}

// GroupCommits groups commits by pattern
func GroupCommits(commits []Commit, groups []CommitGroup) []GroupedCommits {
	var result []GroupedCommits
	used := make(map[string]bool)

	for _, group := range groups {
		gc := GroupedCommits{
			Title: group.Title,
			Order: group.Order,
		}

		re, err := regexp.Compile(group.Regexp)
		if err != nil {
			continue
		}

		for _, c := range commits {
			if used[c.Hash] {
				continue
			}
			if re.MatchString(c.Subject) {
				gc.Commits = append(gc.Commits, c)
				used[c.Hash] = true
			}
		}

		if len(gc.Commits) > 0 {
			result = append(result, gc)
		}
	}

	// Add ungrouped commits
	var ungrouped []Commit
	for _, c := range commits {
		if !used[c.Hash] {
			ungrouped = append(ungrouped, c)
		}
	}
	if len(ungrouped) > 0 {
		result = append(result, GroupedCommits{
			Title:   "Other",
			Commits: ungrouped,
			Order:   999,
		})
	}

	return result
}

// CommitGroup defines a commit group pattern
type CommitGroup struct {
	Title  string
	Regexp string
	Order  int
}

// GroupedCommits represents commits in a group
type GroupedCommits struct {
	Title   string
	Commits []Commit
	Order   int
}

// parseVersion parses a semver tag
func parseVersion(info *Info, tag string) {
	// Strip leading 'v' if present
	version := strings.TrimPrefix(tag, "v")

	// Parse version with semver regex
	re := regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(?:-([a-zA-Z0-9.-]+))?(?:\+([a-zA-Z0-9.-]+))?$`)
	matches := re.FindStringSubmatch(version)
	if len(matches) < 4 {
		return
	}

	fmt.Sscanf(matches[1], "%d", &info.Major)
	fmt.Sscanf(matches[2], "%d", &info.Minor)
	fmt.Sscanf(matches[3], "%d", &info.Patch)

	if len(matches) > 4 {
		info.PrereleaseSuffix = matches[4]
		info.Prerelease = matches[4] != ""
	}
	if len(matches) > 5 {
		info.Metadata = matches[5]
	}
}

// run executes a git command and returns the output
func run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s: %s", err, stderr.String())
	}

	return stdout.String(), nil
}
