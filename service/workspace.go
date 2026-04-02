package service

import (
	"core/config"
	"core/domain"
	"core/models"
	"core/utils"
	"fmt"
	"strconv"
	"time"

	"github.com/dgrijalva/jwt-go"
)

type WorkspaceService struct {
	WorkspaceDomain           domain.WorkspaceDomain
	ManageWorkspaceDomain     domain.ManageWorkspaceDomain
	ChannelDomain             domain.ChannelDomain
	CredentialsDomain         domain.CredentialsDomain
	ManageChannelsDomain      domain.ManageChannelsDomain
	UserDomain                domain.UserDomain
	GitHubRepositoryDomain    domain.GitHubRepositoryDomain
	GitHubInstallationsDomain domain.GitHubInstallationsDomain
	GitHubCommitsDomain       domain.GitHubCommitsDomain
	GitHubCommitFilesDomain   domain.GitHubCommitFilesDomain
}

func (c *WorkspaceService) CreateWorkspace(param models.CreateWorkspaceReqs) (models.CreateWorkspaceResp, error) {
	workspaceParam := models.Workspace{
		Name:    param.Name,
		OwnerID: param.UserID,
	}
	workspace, err := c.WorkspaceDomain.Create(workspaceParam)
	if err != nil {
		return models.CreateWorkspaceResp{}, err
	}
	manageWorkspaceParam := models.ManageWorkspace{
		WorkspaceID:  workspace.ID,
		JoinedUserID: param.UserID,
		Role:         "admin",
		IsAccepted:   true,
	}
	_, err = c.ManageWorkspaceDomain.Create(manageWorkspaceParam)
	if err != nil {
		return models.CreateWorkspaceResp{}, err
	}

	channelParam := models.Channels{
		Name:    "general",
		Type:    true,
		OwnerID: param.UserID,
	}
	channels, err := c.ChannelDomain.Create(channelParam)
	if err != nil {
		return models.CreateWorkspaceResp{}, err
	}
	// docParam := models.Document{
	// 	ChannelID: channels.ID,
	// 	Name:      "This is general channel",
	// }
	// _, err = c.DocumentDomain.Create(docParam)
	// if err != nil {
	// 	return models.CreateWorkspaceResp{}, err
	// }
	// _, err = c.CredentialsDomain.Create(models.Credentials{
	// 	Text:      "This is general channel",
	// 	ChannelID: channels.ID,
	// })
	// if err != nil {
	// 	return models.CreateWorkspaceResp{}, err
	// }
	manageChannelParam := models.ManageChannels{
		ChannelID:    channels.ID,
		JoinedUserID: param.UserID,
		Role:         "admin",
		WorkspaceID:  workspace.ID,
	}
	_, err = c.ManageChannelsDomain.Create(manageChannelParam)
	if err != nil {
		return models.CreateWorkspaceResp{}, err
	}
	resp := models.CreateWorkspaceResp{
		ID:          workspace.ID,
		Name:        workspace.Name,
		RedirectUrl: "/workspace/" + strconv.FormatInt(workspace.ID, 10),
		CreatedAt:   workspace.CreatedAt,
	}
	return resp, nil
}

func (c *WorkspaceService) GetWorkspaceById(param models.GetWorkspaceByIdReqs) (models.GetWorkspaceByIdResp, error) {
	// do you belongs to this workspace
	ManageWorkspace := models.ManageWorkspace{
		WorkspaceID:  param.WorkspaceID,
		JoinedUserID: param.UserID,
	}
	_, err := c.ManageWorkspaceDomain.GetByWorkspaceIdAndUserId(ManageWorkspace)
	if err != nil {
		return models.GetWorkspaceByIdResp{}, fmt.Errorf("you don't belong to this workspace: %w", err)
	}
	workspaceParam := models.Workspace{
		ID: param.WorkspaceID,
	}
	workspace, err := c.WorkspaceDomain.GetById(workspaceParam)
	if err != nil {
		return models.GetWorkspaceByIdResp{}, err
	}
	manageChannelParam := models.ManageChannels{
		WorkspaceID:  param.WorkspaceID,
		JoinedUserID: param.UserID,
	}
	managechannelData, err := c.ManageChannelsDomain.GetByWorkspaceIdAndUser(manageChannelParam)
	if err != nil {
		return models.GetWorkspaceByIdResp{}, fmt.Errorf("you don't belong to this workspace: %w", err)
	}
	resp := models.GetWorkspaceByIdResp{
		WorkspaceID: workspace.ID,
		Workspace:   workspace.Name, // assuming your model has Name
		UserID:      param.UserID,
		Channels:    managechannelData,
	}

	return resp, nil

}

func (c *WorkspaceService) AddUserInWorkspace(param models.AddUserInWorkspaceReqs) (models.BasicResp, error) {
	manageWorkSpaceParam := models.ManageWorkspace{
		WorkspaceID:  param.WorkspaceID,
		JoinedUserID: param.AddedByID,
	}
	userWhoIsAddingRole, err := c.ManageWorkspaceDomain.GetByWorkspaceIdAndUser(manageWorkSpaceParam)
	if err != nil {
		return models.BasicResp{}, fmt.Errorf("failed to get owner id: %w", err)
	}
	if userWhoIsAddingRole.Role != "admin" {
		return models.BasicResp{}, fmt.Errorf("only admin can add user in workspace")
	}
	count, err := c.ManageWorkspaceDomain.CountOfUsersInWorkspace(models.ManageWorkspace{WorkspaceID: param.WorkspaceID})
	if err != nil {
		return models.BasicResp{}, err
	}
	if count >= int64(utils.CountOfUsersInWorkspace) {
		return models.BasicResp{}, fmt.Errorf("you can't add more users in this workspace")
	}
	userParam := models.User{Email: param.UserEmail}
	user, err := c.UserDomain.GetUserByEmail(userParam)
	if err != nil {
		return models.BasicResp{}, err
	}

	existing, err := c.ManageWorkspaceDomain.GetByWorkspaceIdAndUser(models.ManageWorkspace{
		WorkspaceID:  param.WorkspaceID,
		JoinedUserID: user.ID,
	})
	if err == nil {
		if existing.IsAccepted {
			return models.BasicResp{}, fmt.Errorf("user is already a member of this workspace")
		}
		// pending invite — just resend the email, no new record needed
	} else {
		manageWorkspaceParam := models.ManageWorkspace{
			WorkspaceID:  param.WorkspaceID,
			JoinedUserID: user.ID,
			Role:         param.Role,
			IsAccepted:   false,
		}
		_, err = c.ManageWorkspaceDomain.Create(manageWorkspaceParam)
		if err != nil {
			return models.BasicResp{}, err
		}
	}
	// ✅ generate JWT token
	token, err := generateUserToken(int64(user.ID), user.Email)
	if err != nil {
		return models.BasicResp{}, err
	}

	// ✅ invite link
	inviteLink := fmt.Sprintf("%s/invite?jwt=%s&workspaceId=%d", config.GetConfig().FrontendUrl, token, param.WorkspaceID)

	// ✅ prepare email data
	mailData := models.SendMail{
		SendTo: user.Email,
		Data: map[string]interface{}{
			"InviteLink": inviteLink,
			"Workspace":  param.WorkspaceID,
		},
	}

	// ✅ send email with template
	go utils.SendMailForInvite("./template/invite.html", mailData, "You’ve been invited to join a workspace")

	// ✅ return only success message
	return models.BasicResp{
		Message: "Invitation sent successfully",
	}, nil
}

func generateUserToken(userID int64, email string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"email":   email,
		"exp":     time.Now().Add(7 * 24 * time.Hour).Unix(), // 1 week expiry
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.GetConfig().JWTSecret))
}

func (c *WorkspaceService) AcceptInvite(param models.AcceptInviteReqs) (models.BasicResp, error) {
	manageWorkspaceParam := models.ManageWorkspace{
		WorkspaceID:  param.WorkspaceID,
		JoinedUserID: param.UserID,
	}
	manageWorkspace, err := c.ManageWorkspaceDomain.GetByWorkspaceIdAndUserId(manageWorkspaceParam)
	if err != nil {
		return models.BasicResp{}, fmt.Errorf("no invitation found for this user in this workspace: %w", err)
	}
	if manageWorkspace.IsAccepted {
		return models.BasicResp{}, fmt.Errorf("you have already accepted the invitation")
	}
	manageWorkspaceParam.IsAccepted = true
	err = c.ManageWorkspaceDomain.UpdateIsAccepted(manageWorkspaceParam)
	if err != nil {
		return models.BasicResp{}, err
	}
	getPublicChannelParam := models.ManageChannels{
		WorkspaceID: param.WorkspaceID,
	}
	var publicChannels []models.ManageChannels

	publicChannels, err = c.ManageChannelsDomain.GetPublicChannelByWorkspaceId(getPublicChannelParam)
	if err != nil {
		return models.BasicResp{}, err
	}
	for _, channel := range publicChannels {
		manageChannelParam := models.ManageChannels{
			ChannelID:    channel.ChannelID,
			JoinedUserID: param.UserID,
			Role:         "member",
			WorkspaceID:  param.WorkspaceID,
		}
		_, err = c.ManageChannelsDomain.Create(manageChannelParam)
		if err != nil {
			return models.BasicResp{}, err
		}
	}
	return models.BasicResp{
		Message: "Invitation accepted successfully",
	}, nil
}

func (c *WorkspaceService) GetAllWorkspace(userId int64) ([]models.GetAllWorkspaceByUserIdResp, error) {
	data := c.ManageWorkspaceDomain.GetAllWorkspaceByUserId(userId)
	if data == nil {
		return nil, fmt.Errorf("no workspaces found for user: %d", userId)
	}
	return data, nil
}

func (c *WorkspaceService) GetAllRepository(userId, workspaceID int64) ([]models.GitHubRepositoryResponse, error) {
	_, err := c.ManageWorkspaceDomain.GetByWorkspaceIdAndUserId(models.ManageWorkspace{
		WorkspaceID:  workspaceID,
		JoinedUserID: userId,
	})
	if err != nil {
		return nil, fmt.Errorf("access denied")
	}
	repositories, err := c.GitHubInstallationsDomain.GetAllByWorkspaceId(workspaceID)
	if err != nil {
		return nil, err
	}
	var resp []models.GitHubRepositoryResponse
	for _, repo := range repositories {
		resp = append(resp, models.GitHubRepositoryResponse{
			ID:       repo.ID,
			Name:     repo.Name,
			FullName: repo.FullName,
			Private:  repo.Private,
		})
	}

	return resp, nil
}

func (c *WorkspaceService) GetOrgDetails(userId, workspaceID int64) (models.OrgDetailsResponse, error) {
	_, err := c.ManageWorkspaceDomain.GetByWorkspaceIdAndUserId(models.ManageWorkspace{
		WorkspaceID:  workspaceID,
		JoinedUserID: userId,
	})
	if err != nil {
		return models.OrgDetailsResponse{}, fmt.Errorf("access denied")
	}
	data, err := c.GitHubInstallationsDomain.GetOrgDetailsByWorkspaceId(workspaceID)
	if err != nil {
		return models.OrgDetailsResponse{}, err
	}
	return data, nil
}

func (c *WorkspaceService) GetRepoCommits(param models.GetRepoCommitsReqs) (models.GetRepoCommitsPaginatedResponse, error) {
	workspaceID, err := c.GitHubInstallationsDomain.GetWorkspaceIdByRepoId(param.RepoID)
	if err != nil || workspaceID == 0 {
		return models.GetRepoCommitsPaginatedResponse{}, fmt.Errorf("access denied")
	}
	_, err = c.ManageWorkspaceDomain.GetByWorkspaceIdAndUserId(models.ManageWorkspace{
		WorkspaceID:  workspaceID,
		JoinedUserID: param.UserID,
	})
	if err != nil {
		return models.GetRepoCommitsPaginatedResponse{}, fmt.Errorf("access denied")
	}
	commits, err := c.GitHubCommitsDomain.GetRepoCommitsByRepoId(param)
	if err != nil {
		return models.GetRepoCommitsPaginatedResponse{}, err
	}
	return commits, nil
}

func (c *WorkspaceService) GetCommitFilesDetails(userId, commitId int64) ([]models.GitHubCommitFiles, error) {
	workspaceID, err := c.GitHubInstallationsDomain.GetWorkspaceIdByCommitId(commitId)
	if err != nil || workspaceID == 0 {
		return nil, fmt.Errorf("access denied")
	}
	_, err = c.ManageWorkspaceDomain.GetByWorkspaceIdAndUserId(models.ManageWorkspace{
		WorkspaceID:  workspaceID,
		JoinedUserID: userId,
	})
	if err != nil {
		return nil, fmt.Errorf("access denied")
	}
	commitParam := models.GitHubCommitFiles{
		GithubCommitID: commitId,
	}
	data, err := c.GitHubCommitFilesDomain.GetCommitFilesDetailsByCommitId(commitParam)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (c *WorkspaceService) GetWorkspaceDetails(param models.GetWorkspaceDetailsReqs) (models.GetWorkspaceDetailsResp, error) {
	_, err := c.ManageWorkspaceDomain.GetByWorkspaceIdAndUserId(models.ManageWorkspace{
		WorkspaceID:  param.Workspace_id,
		JoinedUserID: param.UserID,
	})
	if err != nil {
		return models.GetWorkspaceDetailsResp{}, fmt.Errorf("access denied")
	}
	workSpacecParam := models.Workspace{
		ID: param.Workspace_id,
	}
	data, err := c.WorkspaceDomain.GetWorkspaceAndUserById(workSpacecParam)
	if err != nil {
		return models.GetWorkspaceDetailsResp{}, err
	}
	return data, nil
}

func (c *WorkspaceService) GetWorkSpaceMembers(param models.GetWorkspaceDetailsReqs) ([]models.WorkspaceMembersResp, error) {
	_, err := c.ManageWorkspaceDomain.GetByWorkspaceIdAndUserId(models.ManageWorkspace{
		WorkspaceID:  param.Workspace_id,
		JoinedUserID: param.UserID,
	})
	if err != nil {
		return nil, fmt.Errorf("access denied")
	}
	workSpacecParam := models.Workspace{
		ID: param.Workspace_id,
	}
	members, err := c.ManageWorkspaceDomain.GetMembersByWorkspaceId(workSpacecParam)
	if err != nil {
		return nil, err
	}
	var resp []models.WorkspaceMembersResp
	for _, member := range members {
		resp = append(resp, models.WorkspaceMembersResp{
			UserID: member.UserID,
			Role:   member.Role,
			Email:  member.Email,
			Name:   member.Name,
		})
	}
	return resp, nil
}
