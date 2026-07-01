package models

import "time"

type GitHubInstallation struct {
	ID             int64     `gorm:"column:id;primary_key"`
	InstallationID int64     `gorm:"column:installation_id;unique;not null"`
	AccountLogin   string    `gorm:"column:account_login;not null"`
	AccountType    string    `gorm:"column:account_type;not null"` // "User" or "Organization"
	UserID         int64     `gorm:"column:user_id"`               // FK to users table
	IsClaimed      bool      `gorm:"column:is_claimed;default:false"`
	WorkspaceID    int64     `gorm:"column:workspace_id;default:null"`
	CreatedAt      time.Time `gorm:"column:created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at"`

	User *User `gorm:"foreignKey:UserID;references:ID"`
}

func (GitHubInstallation) TableName() string {
	return "github_installations"
}

// Request/Response models
type CreateGitHubInstallationReq struct {
	InstallationID int64  `json:"installation_id"`
	AccountLogin   string `json:"account_login"`
	AccountType    string `json:"account_type"`
	UserID         int64  `json:"user_id"`
}

type GitHubInstallationResp struct {
	ID             int64     `json:"id"`
	InstallationID int64     `json:"installation_id"`
	AccountLogin   string    `json:"account_login"`
	AccountType    string    `json:"account_type"`
	UserID         int64     `json:"user_id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type GitHubInstallationByUserReq struct {
	UserID         int64 `json:"user_id"`
	IsClaimed      bool  `json:"is_claimed"`
	InstallationID int64 `json:"installation_id"`
	WorkspaceID    int64 `json:"workspace_id"`
}

type GitHubInstallationEvent struct {
	Action       string `json:"action"`
	Installation struct {
		ID      int64 `json:"id"`
		Account struct {
			Login string `json:"login"`
			Type  string `json:"type"`
		} `json:"account"`
	} `json:"installation"`
}

type GenerateInstallationTokenReq struct {
	ID     int64 `json:"id"`
	UserID int64 `json:"user_id"`
}

type GitHubCommitFile struct {
	Filename  string `json:"filename"`
	Status    string `json:"status"` // added, modified, removed
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Patch     string `json:"patch"` // THIS IS GOLD
}

type GitHubCommitDetail struct {
	SHA    string             `json:"sha"`
	Files  []GitHubCommitFile `json:"files"`
	Commit struct {
		Message string `json:"message"`
		Author  struct {
			Name  string `json:"name"`
			Email string `json:"email"`
			Date  string `json:"date"`
		} `json:"author"`
	} `json:"commit"`
}

type GitHubCommit struct {
	SHA string `json:"sha"`

	Commit struct {
		Message string `json:"message"`

		Author struct {
			Name  string `json:"name"`
			Email string `json:"email"`
			Date  string `json:"date"`
		} `json:"author"`
	} `json:"commit"`

	Author *struct {
		Login string `json:"login"`
		ID    int64  `json:"id"`
	} `json:"author"`

	HTMLURL string `json:"html_url"`
}

type CommitDetailResponse struct {
	SHA         string             `json:"sha"`
	Message     string             `json:"message"`
	Author      string             `json:"author"`
	AuthorEmail string             `json:"author_email"`
	Date        string             `json:"date"`
	Files       []GitHubCommitFile `json:"files"`
}

type RepositoryCommitsResponse struct {
	RepoID       int64                  `json:"repo_id"`
	RepoName     string                 `json:"repo_name"`
	RepoFullName string                 `json:"repo_full_name"`
	Private      bool                   `json:"private"`
	Commits      []CommitDetailResponse `json:"commits"`
}

type AllReposWithCommitsResponse struct {
	TotalRepositories int                         `json:"total_repositories"`
	Repositories      []RepositoryCommitsResponse `json:"repositories"`
}

type GitHubRepo struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Private  bool   `json:"private"`
	HTMLURL  string `json:"html_url"`
}

type InstallationReposResponse struct {
	TotalCount   int          `json:"total_count"`
	Repositories []GitHubRepo `json:"repositories"`
}

type OrgDetailsResponse struct {
	ID             int64                      `json:"id"`
	InstallationID int64                      `json:"installation_id"`
	AccountLogin   string                     `json:"account_login"`
	AccountType    string                     `json:"account_type"`
	WorkspaceID    int64                      `json:"workspace_id"`
	Repositories   []GitHubRepositoryResponse `json:"repositories"`
}

// GitHubInstallationRepoRef represents a repository entry in installation_repositories events.
type GitHubInstallationRepoRef struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Private  bool   `json:"private"`
}

// GitHubInstallationRepositoriesEvent is the webhook payload for the
// "installation_repositories" event fired by GitHub when repos are added
// or removed from an installation.
type GitHubInstallationRepositoriesEvent struct {
	Action       string `json:"action"` // "added" | "removed"
	Installation struct {
		ID      int64 `json:"id"`
		Account struct {
			Login string `json:"login"`
			Type  string `json:"type"`
		} `json:"account"`
	} `json:"installation"`
	RepositoriesAdded   []GitHubInstallationRepoRef `json:"repositories_added"`
	RepositoriesRemoved []GitHubInstallationRepoRef `json:"repositories_removed"`
}
