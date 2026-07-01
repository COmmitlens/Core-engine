package domain

import (
	"core/config"
	"core/models"
	"fmt"
)

type GitHubRepositoryDomain interface {
	StoreRepository(params models.GitHubRepository) (int64, error)
	GetAllByWorkspaceId(workspaceID int64) ([]models.GitHubRepository, error)
	GetRepositoryActivity(repoID, days int64) ([]models.CommitActivity, error)
	FindRepositoryByInstallationID(params models.GitHubRepository) (models.GitHubRepository, error)
	FindUserIdByInstallationID(params models.GitHubRepository) (int64, error)
	GetByID(id int64) (models.GitHubRepository, error)
	DeleteByGithubRepoID(githubRepoID int64) error
}

type GitHubRepositoryDomainCtx struct{}

func (g *GitHubRepositoryDomainCtx) StoreRepository(params models.GitHubRepository) (int64, error) {
	db := config.DbManager()

	err := db.Create(&params).Error

	if err != nil {
		return 0, err
	}

	return params.ID, nil

}

func (g *GitHubRepositoryDomainCtx) GetAllByWorkspaceId(workspaceID int64) ([]models.GitHubRepository, error) {
	db := config.DbManager()
	var repositories []models.GitHubRepository

	err := db.Where("installation_id = ?", workspaceID).Find(&repositories).Error
	if err != nil {
		return nil, err
	}

	return repositories, nil
}

func (g *GitHubRepositoryDomainCtx) GetRepositoryActivity(
	repoID, days int64,
) ([]models.CommitActivity, error) {

	db := config.DbManager()

	var activities []models.CommitActivity

	err := db.
		Table("git_hub_commits c").
		Select(`
		c.id AS commit_id,
		c.commit_sha,
		c.commit_message,
		c.github_author_name,
		DATE(c.committed_at) AS commit_date,
		COALESCE(
			ARRAY_AGG(f.filename) FILTER (WHERE f.filename IS NOT NULL),
			'{}'
		) AS files_changed
	`).
		Joins("LEFT JOIN git_hub_commit_files f ON f.github_commit_id = c.id").
		Where(
			"c.github_repository_id = ? AND c.committed_at >= NOW() - make_interval(days => ?)",
			repoID,
			days,
		).
		Group("c.id, commit_date").
		Order("commit_date DESC, c.committed_at DESC").
		Scan(&activities).Error

	if err != nil {
		return nil, err
	}

	return activities, nil
}

func (g *GitHubRepositoryDomainCtx) FindRepositoryByInstallationID(params models.GitHubRepository) (models.GitHubRepository, error) {
	db := config.DbManager()
	var repository models.GitHubRepository

	err := db.Where("installation_id = ? and github_repo_id = ?", params.InstallationID, params.GithubRepoID).First(&repository).Error
	if err != nil {
		return models.GitHubRepository{}, err
	}

	return repository, nil
}

func (g *GitHubRepositoryDomainCtx) GetByID(id int64) (models.GitHubRepository, error) {
	db := config.DbManager()
	var repository models.GitHubRepository
	err := db.First(&repository, id).Error
	if err != nil {
		return models.GitHubRepository{}, err
	}
	return repository, nil
}

func (g *GitHubRepositoryDomainCtx) FindUserIdByInstallationID(params models.GitHubRepository) (int64, error) {
	db := config.DbManager()
	var repository models.GitHubRepository

	err := db.Where("installation_id = ? ", params.InstallationID).First(&repository).Error
	if err != nil {
		return 0, err
	}

	return repository.UserID, nil
}

// DeleteByGithubRepoID removes a repository and all its associated commits, commit
// files, and embeddings identified by the GitHub repository ID.
func (g *GitHubRepositoryDomainCtx) DeleteByGithubRepoID(githubRepoID int64) error {
	db := config.DbManager()

	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Find the internal repo record
	var repo models.GitHubRepository
	if err := tx.Where("github_repo_id = ?", githubRepoID).First(&repo).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("repo with github_repo_id %d not found: %w", githubRepoID, err)
	}

	// Collect commit IDs for this repo
	var commitIDs []int64
	if err := tx.Model(&models.GitHubCommits{}).
		Where("github_repository_id = ?", repo.ID).
		Pluck("id", &commitIDs).Error; err != nil {
		tx.Rollback()
		return err
	}

	if len(commitIDs) > 0 {
		// Collect commit file IDs
		var fileIDs []int64
		if err := tx.Model(&models.GitHubCommitFiles{}).
			Where("github_commit_id IN ?", commitIDs).
			Pluck("id", &fileIDs).Error; err != nil {
			tx.Rollback()
			return err
		}

		if len(fileIDs) > 0 {
			// Delete embeddings for those files
			if err := tx.Where("commit_file_id IN ?", fileIDs).
				Delete(&models.CommitFileEmbedding{}).Error; err != nil {
				tx.Rollback()
				return err
			}
			// Delete commit files
			if err := tx.Where("id IN ?", fileIDs).
				Delete(&models.GitHubCommitFiles{}).Error; err != nil {
				tx.Rollback()
				return err
			}
		}

		// Delete commits
		if err := tx.Where("id IN ?", commitIDs).
			Delete(&models.GitHubCommits{}).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	// Delete the repository itself
	if err := tx.Where("github_repo_id = ?", githubRepoID).
		Delete(&models.GitHubRepository{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}
