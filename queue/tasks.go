package queue

// Task type constants for asynq queue
const (
	TypeFetchAndStoreRepos                  = "github:fetch_and_store_repos"
	TypeEmbedCommitFile                     = "github:embed_commit_file"
	TypeEmbedCommitFileV2                   = "github:embed_commit_file_v2"
	TypeHandlePushEvent                     = "github:handle_push_event"
	TypeHandleInstallationRepositoriesEvent = "github:handle_installation_repositories_event"
)

// FetchAndStoreReposPayload is the payload for the fetch-and-store-repos task
type FetchAndStoreReposPayload struct {
	InstallationID int64 `json:"installation_id"`
	UserID         int64 `json:"user_id"`
	WorkspaceID    int64 `json:"workspace_id"`
}

// EmbedCommitFilePayload is the payload for the embed-commit-file task
type EmbedCommitFilePayload struct {
	RepoFullName  string `json:"repo_full_name"`
	RepoGithubID  int64  `json:"repo_github_id"`
	RepoInstallID int64  `json:"repo_install_id"`
	CommitID      int64  `json:"commit_id"`
	CommitSHA     string `json:"commit_sha"`
	CommitMessage string `json:"commit_message"`
	AuthorName    string `json:"author_name"`
	FileID        int64  `json:"file_id"`
	Filename      string `json:"filename"`
	Patch         string `json:"patch"`
}

// EmbedCommitFileV2Payload mirrors models.EmbedCommitFile2
type EmbedCommitFileV2Payload struct {
	CommitFileID   int64  `json:"commit_file_id"`
	GitHubRepoID   int64  `json:"github_repo_id"`
	InstallationID int64  `json:"installation_id"`
	GithubCommitID int64  `json:"github_commit_id"`
	FullName       string `json:"full_name"`
	Filename       string `json:"filename"`
	Author         string `json:"author"`
	Message        string `json:"message"`
	Patch          string `json:"patch"`
}

// HandlePushEventPayload wraps the raw GitHub push webhook JSON
type HandlePushEventPayload struct {
	RawJSON []byte `json:"raw_json"`
}

// HandleInstallationRepositoriesPayload wraps the raw installation_repositories webhook JSON
type HandleInstallationRepositoriesPayload struct {
	RawJSON []byte `json:"raw_json"`
}
