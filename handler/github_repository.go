package handler

import (
	"core/models"
	"core/service"
	"net/http"
	"strconv"

	"github.com/labstack/echo"
)

type GitHubRepositoryHandler struct {
	GitHubRepositoryService service.GitHubRepositoryService
}

func (githubRepositoryHandler *GitHubRepositoryHandler) GetRepositoryActivity(c echo.Context) error {
	repoID := c.Param("repo_id")
	days := c.QueryParam("days")
	if days == "" {
		days = "7"
	}
	repoIDInt, err := strconv.ParseInt(repoID, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid repository id"})
	}
	daysInt, err := strconv.ParseInt(days, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid days parameter"})
	}
	activity, err := githubRepositoryHandler.GitHubRepositoryService.GetRepositoryActivity(repoIDInt, daysInt)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, activity)
}

func (githubRepositoryHandler *GitHubRepositoryHandler) GetCommitDetails(c echo.Context) error {
	// Implementation for fetching commit details goes here
	commitSHA := c.Param("commit_sha")
	if commitSHA == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid commit sha"})
	}
	repoID := c.Param("repo_id")
	repoIDInt, err := strconv.ParseInt(repoID, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid repository id"})
	}
	commitDetails, err := githubRepositoryHandler.GitHubRepositoryService.GetCommitDetails(repoIDInt, commitSHA)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, commitDetails)
}

func (githubRepositoryHandler *GitHubRepositoryHandler) GetRelatedCommitFiles(c echo.Context) error {
	commitFileID := c.Param("commit_file_id")
	if commitFileID == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid commit file id"})
	}
	relatedFiles, err := githubRepositoryHandler.GitHubRepositoryService.GetRelatedCommitFiles(commitFileID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, relatedFiles)
}

func (githubRepositoryHandler *GitHubRepositoryHandler) ExplainCommitFileChange(c echo.Context) error {
	commitFileID := c.Param("commit_file_id")
	if commitFileID == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid commit file id"})
	}

	param := models.ExplainCommitFileChangeRequest{}
	if err := c.Bind(&param); err != nil {
		return c.JSON(400, models.BasicResp{Message: err.Error()})
	}
	param.CommitFileID = commitFileID
	explainedAnswer, err := githubRepositoryHandler.GitHubRepositoryService.ExplainCommitFileChange(param)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, explainedAnswer)
}

func (githubRepositoryHandler *GitHubRepositoryHandler) QueryWorkspace(c echo.Context) error {
	workspaceID := c.Param("workspace_id")
	workspaceIDInt, err := strconv.ParseInt(workspaceID, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid workspace_id"})
	}

	var param models.WorkspaceQueryRequest
	if err := c.Bind(&param); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	if param.Query == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "query is required"})
	}

	response, err := githubRepositoryHandler.GitHubRepositoryService.QueryWorkspace(param, workspaceIDInt)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, response)
}

// BackfillEmbeddings queues embedding tasks for all commit files that have no
// embedding yet. Useful for seeding historical data.
func (githubRepositoryHandler *GitHubRepositoryHandler) BackfillEmbeddings(c echo.Context) error {
	count, err := githubRepositoryHandler.GitHubRepositoryService.BackfillEmbeddings()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, echo.Map{
		"message":  "backfill started",
		"enqueued": count,
	})
}
