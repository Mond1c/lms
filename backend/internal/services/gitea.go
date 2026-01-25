package services

import (
	"fmt"

	"code.gitea.io/sdk/gitea"
)

type GiteaService struct {
	client  *gitea.Client
	baseURL string
}

func NewGiteaService(baseURL, token string) (*GiteaService, error) {
	client, err := gitea.NewClient(baseURL, gitea.SetToken(token))
	if err != nil {
		return nil, err
	}

	return &GiteaService{client: client, baseURL: baseURL}, nil
}

func (s *GiteaService) CreateFork(owner, repo, forkOwner string) (*gitea.Repository, error) {
	opts := gitea.CreateForkOption{
		Organization: &forkOwner,
	}

	repository, _, err := s.client.CreateFork(owner, repo, opts)
	if err != nil {
		return nil, err
	}

	return repository, nil
}

func (s *GiteaService) GetUser() (*gitea.User, error) {
	user, _, err := s.client.GetMyUserInfo()
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *GiteaService) GetUserByUsername(username string) (*gitea.User, error) {
	user, _, err := s.client.GetUserInfo(username)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *GiteaService) CreateOrganization(name, description string) (*gitea.Organization, error) {
	opts := gitea.CreateOrgOption{
		Name:        name,
		Description: description,
	}

	org, _, err := s.client.CreateOrg(opts)
	if err != nil {
		return nil, err
	}
	return org, nil
}

func (s *GiteaService) CreateRepoFromTemplate(orgName, templateOwner, templateRepo, newRepoName string, private bool) (*gitea.Repository, error) {
	opts := gitea.CreateRepoFromTemplateOption{
		Owner:       orgName,
		Name:        newRepoName,
		Private:     private,
		Description: fmt.Sprintf("Assignment repository for %s", newRepoName),
	}

	repo, _, err := s.client.CreateRepoFromTemplate(templateOwner, templateRepo, opts)
	if err != nil {
		return nil, err
	}
	return repo, nil
}

func (s *GiteaService) CreateOrgRepo(orgName, repoName, description string, private bool) (*gitea.Repository, error) {
	opts := gitea.CreateRepoOption{
		Name:        repoName,
		Description: description,
		Private:     private,
		AutoInit:    true,
	}

	repo, _, err := s.client.CreateOrgRepo(orgName, opts)
	if err != nil {
		return nil, err
	}
	return repo, nil
}

func (s *GiteaService) AddCollaborator(owner, repo, username string, permission gitea.AccessMode) error {
	opts := gitea.AddCollaboratorOption{
		Permission: &permission,
	}
	_, err := s.client.AddCollaborator(owner, repo, username, opts)
	return err
}

func (s *GiteaService) GetOrgTeams(orgName string) ([]*gitea.Team, error) {
	teams, _, err := s.client.ListOrgTeams(orgName, gitea.ListTeamsOptions{})
	if err != nil {
		return nil, err
	}
	return teams, nil
}

func (s *GiteaService) CreateTeam(orgName, teamName, description string, permission gitea.AccessMode) (*gitea.Team, error) {
	opts := gitea.CreateTeamOption{
		Name:        teamName,
		Description: description,
		Permission:  permission,
	}

	team, _, err := s.client.CreateTeam(orgName, opts)
	if err != nil {
		return nil, err
	}
	return team, nil
}

func (s *GiteaService) AddTeamMember(teamID int64, username string) error {
	_, err := s.client.AddTeamMember(teamID, username)
	return err
}

func (s *GiteaService) AddTeamRepository(teamID int64, orgName, repoName string) error {
	_, err := s.client.AddTeamRepository(teamID, orgName, repoName)
	return err
}

func (s *GiteaService) GetRepo(owner, repoName string) (*gitea.Repository, error) {
	repo, _, err := s.client.GetRepo(owner, repoName)
	if err != nil {
		return nil, err
	}
	return repo, nil
}

func (s *GiteaService) GetRepoURL(owner, repoName string) string {
	return fmt.Sprintf("%s/%s/%s", s.baseURL, owner, repoName)
}

func (s *GiteaService) GetRepoBranch(owner, repo, branch string) (*gitea.Branch, error) {
	b, _, err := s.client.GetRepoBranch(owner, repo, branch)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (s *GiteaService) CreateBranch(owner, repo, branchName, fromBranch string) error {
	opts := gitea.CreateBranchOption{
		BranchName:    branchName,
		OldBranchName: fromBranch,
	}
	_, _, err := s.client.CreateBranch(owner, repo, opts)
	return err
}

func (s *GiteaService) CreatePullRequest(owner, repo, title, body, head, base string) (*gitea.PullRequest, error) {
	opts := gitea.CreatePullRequestOption{
		Title: title,
		Body:  body,
		Head:  head,
		Base:  base,
	}
	pr, _, err := s.client.CreatePullRequest(owner, repo, opts)
	if err != nil {
		return nil, err
	}
	return pr, nil
}

func (s *GiteaService) SetupFeedbackBranch(owner, repo string) error {
	err := s.CreateBranch(owner, repo, "feedback", "main")
	if err != nil {
		return err
	}

	_, err = s.CreatePullRequest(
		owner,
		repo,
		"Feedback",
		"This pull request is used by instructors to provide feedback on your work.\n\n**Do not merge or close this PR.**",
		"main",
		"feedback",
	)
	return err
}
