package domain

import (
	"core/config"
	"core/models"
)

type GitHubCommitFilesDomain interface {
	StoreCommitFile(params models.GitHubCommitFiles) (int64, error)
	GetGitHubCommitFilesByID(commitFileID int64) (models.GitHubCommitFiles, error)
	GetCommitFilesDetailsByCommitId(param models.GitHubCommitFiles) ([]models.GitHubCommitFiles, error)
	GetUnembeddedCommitFiles() ([]models.BackfillCommitFileRow, error)
	GetCommitFileHistory(repoID int64, filename string) ([]models.CommitFileHistory, error)
}

type GitHubCommitFilesDomainCtx struct{}

func (g *GitHubCommitFilesDomainCtx) StoreCommitFile(params models.GitHubCommitFiles) (int64, error) {
	db := config.DbManager()

	err := db.Create(&params).Error

	if err != nil {
		return 0, err
	}

	return params.ID, nil

}

func (g *GitHubCommitFilesDomainCtx) GetGitHubCommitFilesByID(commitFileID int64) (models.GitHubCommitFiles, error) {
	db := config.DbManager()

	var commitFile models.GitHubCommitFiles

	err := db.Where("id = ?", commitFileID).First(&commitFile).Error

	if err != nil {
		return models.GitHubCommitFiles{}, err
	}

	return commitFile, nil

}

func (g *GitHubCommitFilesDomainCtx) GetCommitFilesDetailsByCommitId(param models.GitHubCommitFiles) ([]models.GitHubCommitFiles, error) {
	db := config.DbManager()

	var commitFileDetails []models.GitHubCommitFiles

	err := db.Where("github_commit_id = ?", param.GithubCommitID).Find(&commitFileDetails).Error

	if err != nil {
		return nil, err
	}

	return commitFileDetails, nil
}

func (g *GitHubCommitFilesDomainCtx) GetCommitFileHistory(repoID int64, filename string) ([]models.CommitFileHistory, error) {
	db := config.DbManager()

	var rows []models.CommitFileHistory
	err := db.Raw(`
		SELECT
			f.id                     AS commit_file_id,
			c.commit_sha,
			c.github_author_name     AS author,
			c.committed_at,
			c.commit_message         AS message,
			f.status,
			f.additions,
			f.deletions,
			f.patch
		FROM git_hub_commit_files f
		JOIN git_hub_commits c ON c.id = f.github_commit_id
		WHERE f.github_repo_id = ?
		  AND f.filename = ?
		ORDER BY c.committed_at DESC
	`, repoID, filename).Scan(&rows).Error

	return rows, err
}

// GetUnembeddedCommitFiles returns all commit files that have a usable patch
// but no row yet in commit_file_embedding.
func (g *GitHubCommitFilesDomainCtx) GetUnembeddedCommitFiles() ([]models.BackfillCommitFileRow, error) {
	db := config.DbManager()

	var rows []models.BackfillCommitFileRow
	err := db.Raw(`
		SELECT
			f.id                       AS commit_file_id,
			f.github_commit_id,
			f.github_repo_id,
			r.installation_id,
			r.full_name,
			f.filename,
			f.patch,
			c.github_author_name       AS author,
			c.commit_message           AS message
		FROM git_hub_commit_files f
		JOIN git_hub_commits       c ON c.id  = f.github_commit_id
		JOIN git_hub_repository    r ON r.github_repo_id = f.github_repo_id
		LEFT JOIN commit_file_embedding e ON e.commit_file_id = f.id
		WHERE e.commit_file_id IS NULL
		  AND f.status   != 'removed'
		  AND TRIM(f.patch) != ''
		  AND LENGTH(f.patch) <= 6000
	`).Scan(&rows).Error

	return rows, err
}
