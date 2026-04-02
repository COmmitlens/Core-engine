package models

import "time"

type GitHubCommits struct {
	ID                 int64     `gorm:"column:id;primaryKey"`
	GithubRepositoryID int64     `gorm:"column:github_repository_id;index;not null"`
	CommitSHA          string    `gorm:"column:commit_sha;uniqueIndex;not null"`
	GithubAuthorName   string    `gorm:"column:github_author_name;index;not null"`
	AuthorEmail        string    `gorm:"column:author_email"`
	CommitMessage      string    `gorm:"column:commit_message;type:text;not null"`
	CommittedAt        time.Time `gorm:"column:committed_at;index;not null"`
	CreatedAt          time.Time `gorm:"column:created_at;autoCreateTime"`
}

type GitHubCommitResponse struct {
	Id               int64     `json:"id"`
	Sha              string    `json:"sha"`
	Message          string    `json:"message"`
	GitHubAuthorName string    `gorm:"column:github_author_name" json:"github_author_name"`
	AuthorEmail      string    `json:"author_email"`
	CommittedAt      time.Time `json:"committed_at"`
}

type GetRepoCommitsPaginatedResponse struct {
	Commits []GitHubCommitResponse `json:"commits"`
	Meta    MetaPagination         `json:"meta"`
}
