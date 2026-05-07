package domain

import (
	"core/config"
	"core/models"
	"core/utils"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"gorm.io/gorm"
)

type ConnectOrgDomain interface {
	GenerateGitHubAppJWT() (string, error)
	UpdateInstallationByUser(params models.GitHubInstallation) (string, error)
	StoreInstallation(params models.GitHubInstallation) (string, error)
	FindInstallationByInstallationID(installationID int64) (*models.GitHubInstallation, error)
	UpdateInstallationMetadata(installationID int64, accountLogin string, accountType string) error
	FindInstallationByID(Id int64) (*models.GitHubInstallation, error)
	GenerateInstallationToken(appJwt string, url string) (string, error)
	FetchAllRepositoriesWithCommits(installationToken string) (models.AllReposWithCommitsResponse, error)
	FetchCommitDetail(token string, fullRepoName string, sha string) (*models.GitHubCommitDetail, error)
	DeleteInstallationAndData(installationID int64) error
}

type ConnectOrgDomainCtx struct {
}

func (c *ConnectOrgDomainCtx) GenerateGitHubAppJWT() (string, error) {
	appJWT, err := utils.GenerateGitHubAppJWT()
	if err != nil {
		return "", err
	}
	installationID := int64(35045628) // Replace with your GitHub App Installation ID

	url := fmt.Sprintf(
		"https://api.github.com/app/installations/%d/access_tokens",
		installationID,
	)

	req, _ := http.NewRequest("POST", url, nil)
	req.Header.Set("Authorization", "Bearer "+appJWT)
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("github error: %s", string(body))
	}

	var result struct {
		Token string `json:"token"`
	}

	json.NewDecoder(resp.Body).Decode(&result)

	return result.Token, nil
}

func (c *ConnectOrgDomainCtx) UpdateInstallationByUser(param models.GitHubInstallation) (string, error) {
	db := config.DbManager()

	// Verify the user exists before attempting the update to avoid FK violations
	var user models.User
	if err := db.Where("id = ?", param.UserID).First(&user).Error; err != nil {
		return "", fmt.Errorf("user with id %d not found: please log in again", param.UserID)
	}

	// Start a transaction
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Mark all previous installations by this user as inactive
	if err := tx.Model(&models.GitHubInstallation{}).
		Where("user_id = ? AND is_claimed = true", param.UserID).
		Update("is_claimed", false).Error; err != nil {
		tx.Rollback()
		return "", err
	}

	// Claim the new installation
	result := tx.Model(&models.GitHubInstallation{}).
		Where("installation_id = ? AND is_claimed = false", param.InstallationID).
		Updates(map[string]interface{}{
			"user_id":      param.UserID,
			"is_claimed":   true,
			"workspace_id": param.WorkspaceID,
		})

	if result.Error != nil {
		tx.Rollback()
		return "", result.Error
	}

	if result.RowsAffected == 0 {
		tx.Rollback()
		return "", fmt.Errorf("installation with id %d not found or already claimed", param.InstallationID)
	}

	if err := tx.Commit().Error; err != nil {
		return "", err
	}

	return "GitHub installation updated successfully", nil
}

func (c *ConnectOrgDomainCtx) StoreInstallation(param models.GitHubInstallation) (string, error) {
	db := config.DbManager()

	var existingInstallation models.GitHubInstallation
	result := db.Where("installation_id = ?", param.InstallationID).First(&existingInstallation)
	if result.Error == nil {
		// Installation already exists, no need to create a new one
		return "Installation already exists", nil
	} else if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
		// An unexpected error occurred
		return "", result.Error
	}

	// Create a new installation record
	newInstallation := models.GitHubInstallation{
		InstallationID: param.InstallationID,
		AccountLogin:   param.AccountLogin,
		AccountType:    param.AccountType,
		UserID:         param.UserID,
		IsClaimed:      param.IsClaimed,
		WorkspaceID:    param.WorkspaceID,
	}

	// Omit user_id and workspace_id when they are 0 (unclaimed installation from webhook).
	// Inserting 0 would violate the FK constraint; NULL is the correct value for unclaimed rows.
	createQuery := db
	if newInstallation.UserID == 0 {
		createQuery = createQuery.Omit("UserID")
	}
	if newInstallation.WorkspaceID == 0 {
		createQuery = createQuery.Omit("WorkspaceID")
	}

	if err := createQuery.Create(&newInstallation).Error; err != nil {
		return "", err
	}

	return "Installation stored successfully", nil
}

func (c *ConnectOrgDomainCtx) FindInstallationByInstallationID(installationID int64) (*models.GitHubInstallation, error) {
	db := config.DbManager()

	var installation models.GitHubInstallation
	result := db.Where("installation_id = ?", installationID).First(&installation)
	if result.Error != nil {
		return nil, nil
	}

	return &installation, nil
}

// DeleteInstallationAndData removes the installation record and all associated repositories,
// commits, and commit files. Called when a GitHub App installation is uninstalled.
func (c *ConnectOrgDomainCtx) DeleteInstallationAndData(installationID int64) error {
	db := config.DbManager()

	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Find all repos for this installation to get their IDs
	var repoIDs []int64
	if err := tx.Model(&models.GitHubRepository{}).
		Where("installation_id = ?", installationID).
		Pluck("id", &repoIDs).Error; err != nil {
		tx.Rollback()
		return err
	}

	if len(repoIDs) > 0 {
		// Find all commit IDs for these repos
		var commitIDs []int64
		if err := tx.Model(&models.GitHubCommits{}).
			Where("github_repository_id IN ?", repoIDs).
			Pluck("id", &commitIDs).Error; err != nil {
			tx.Rollback()
			return err
		}

		if len(commitIDs) > 0 {
			// Delete commit files
			if err := tx.Where("github_commit_id IN ?", commitIDs).
				Delete(&models.GitHubCommitFiles{}).Error; err != nil {
				tx.Rollback()
				return err
			}
			// Delete commits
			if err := tx.Where("id IN ?", commitIDs).
				Delete(&models.GitHubCommits{}).Error; err != nil {
				tx.Rollback()
				return err
			}
		}

		// Delete repositories
		if err := tx.Where("id IN ?", repoIDs).
			Delete(&models.GitHubRepository{}).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	// Delete the installation record itself
	if err := tx.Where("installation_id = ?", installationID).
		Delete(&models.GitHubInstallation{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

func (c *ConnectOrgDomainCtx) UpdateInstallationMetadata(installationID int64, accountLogin string, accountType string) error {
	db := config.DbManager()
	result := db.Model(&models.GitHubInstallation{}).
		Where("installation_id = ?", installationID).
		Updates(map[string]interface{}{
			"account_login": accountLogin,
			"account_type":  accountType,
		})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("no installation found with id %d", installationID)
	}

	return nil
}

func (c *ConnectOrgDomainCtx) FindInstallationByID(Id int64) (*models.GitHubInstallation, error) {
	db := config.DbManager()

	var installation models.GitHubInstallation
	result := db.Where("id = ?", Id).First(&installation)
	if result.Error != nil {
		return nil, nil
	}

	return &installation, nil
}

func (c *ConnectOrgDomainCtx) GenerateInstallationToken(appJwt string, url string) (string, error) {
	req, _ := http.NewRequest("POST", url, nil)
	req.Header.Set("Authorization", "Bearer "+appJwt)
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("github error: %s", string(body))
	}

	var result struct {
		Token string `json:"token"`
	}

	json.NewDecoder(resp.Body).Decode(&result)

	return result.Token, nil
}

func (c *ConnectOrgDomainCtx) FetchAllRepositoriesWithCommits(installationToken string) (models.AllReposWithCommitsResponse, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/installation/repositories",
		nil)
	if err != nil {
		return models.AllReposWithCommitsResponse{}, err
	}

	req.Header.Set("Authorization", "Bearer "+installationToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return models.AllReposWithCommitsResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return models.AllReposWithCommitsResponse{}, fmt.Errorf("githhub api error: %s", body)
	}

	var repoResult models.InstallationReposResponse
	if err := json.NewDecoder(resp.Body).Decode(&repoResult); err != nil {
		return models.AllReposWithCommitsResponse{}, err
	}

	allReposWithCommits := models.AllReposWithCommitsResponse{
		TotalRepositories: repoResult.TotalCount,
		Repositories:      make([]models.RepositoryCommitsResponse, 0),
	}
	// return allReposWithCommits, nil
	for _, repo := range repoResult.Repositories {
		fmt.Printf("Processing repo: %s\n", repo.FullName)

		commits, err := c.FetchRecentCommits(installationToken, repo.FullName)
		if err != nil {
			fmt.Printf("Error fetching commits for %s: %v\n", repo.FullName, err)
			continue
		}

		repoCommits := models.RepositoryCommitsResponse{
			RepoID:       repo.ID,
			RepoName:     repo.Name,
			RepoFullName: repo.FullName,
			Private:      repo.Private,
			Commits:      make([]models.CommitDetailResponse, 0),
		}

		for _, commit := range commits {
			commitDetail, err := c.FetchCommitDetail(installationToken, repo.FullName, commit.SHA)
			if err != nil {
				fmt.Printf("Error fetching commit detail for %s: %v\n", commit.SHA, err)
				continue
			}

			repoCommits.Commits = append(repoCommits.Commits, models.CommitDetailResponse{
				SHA:         commitDetail.SHA,
				Message:     commitDetail.Commit.Message,
				Author:      commitDetail.Commit.Author.Name,
				AuthorEmail: commitDetail.Commit.Author.Email,
				Date:        commitDetail.Commit.Author.Date,
				Files:       commitDetail.Files,
			})
		}

		allReposWithCommits.Repositories = append(allReposWithCommits.Repositories, repoCommits)
	}

	return allReposWithCommits, nil
}

func (c *ConnectOrgDomainCtx) FetchRecentCommits(token string, fullRepoName string) ([]models.GitHubCommit, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/commits?per_page=30", fullRepoName)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github error: %s", body)
	}

	var commits []models.GitHubCommit
	err = json.NewDecoder(resp.Body).Decode(&commits)
	return commits, err
}

func (c *ConnectOrgDomainCtx) FetchCommitDetail(token string, fullRepoName string, sha string) (*models.GitHubCommitDetail, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/commits/%s", fullRepoName, sha)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github error: %s", body)
	}

	var detail models.GitHubCommitDetail
	err = json.NewDecoder(resp.Body).Decode(&detail)
	return &detail, err
}
