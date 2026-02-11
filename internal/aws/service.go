package aws

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	identitytypes "github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	orgtypes "github.com/aws/aws-sdk-go-v2/service/organizations/types"
	"github.com/aws/aws-sdk-go-v2/service/ssoadmin"
	ssoadmintypes "github.com/aws/aws-sdk-go-v2/service/ssoadmin/types"
)

type Instance struct {
	ARN            string
	IdentityStore  string
	DisplayName    string
	IdentitySource string
}

type Group struct {
	ID          string
	DisplayName string
	Description string
}

type GroupUser struct {
	MembershipID string
	UserID       string
	DisplayName  string
	Email        string
}

type User struct {
	ID          string
	DisplayName string
	UserName    string
	Email       string
}

type Account struct {
	ID    string
	Name  string
	Email string
}

type PermissionSet struct {
	ARN  string
	Name string
}

type Assignment struct {
	AccountID         string
	AccountName       string
	PermissionSetARN  string
	PermissionSetName string
}

type Service struct {
	profile string
	region  string

	identityStoreID string
	instanceARN     string

	identityClient *identitystore.Client
	ssoAdminClient *ssoadmin.Client
	orgClient      *organizations.Client
}

func NewService(profile, region string) *Service {
	return &Service{
		profile: profile,
		region:  region,
	}
}

func (s *Service) EnsureSession(ctx context.Context) ([]Instance, error) {
	if err := s.loadClients(ctx); err != nil {
		return nil, err
	}

	instances, err := s.listInstances(ctx)
	if err == nil {
		return instances, nil
	}

	if !isSSOAuthError(err) {
		return nil, err
	}

	if loginErr := s.loginSSO(ctx); loginErr != nil {
		return nil, fmt.Errorf("aws sso login failed: %w", loginErr)
	}

	if err := s.loadClients(ctx); err != nil {
		return nil, err
	}

	instances, err = s.listInstances(ctx)
	if err != nil {
		return nil, err
	}

	return instances, nil
}

func (s *Service) SetInstance(instanceARN, identityStoreID string) {
	s.instanceARN = instanceARN
	s.identityStoreID = identityStoreID
}

func (s *Service) ListGroups(ctx context.Context) ([]Group, error) {
	groups := make([]Group, 0, 32)
	pager := identitystore.NewListGroupsPaginator(s.identityClient, &identitystore.ListGroupsInput{
		IdentityStoreId: &s.identityStoreID,
	})

	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, g := range page.Groups {
			groups = append(groups, Group{
				ID:          value(g.GroupId),
				DisplayName: value(g.DisplayName),
				Description: value(g.Description),
			})
		}
	}

	return groups, nil
}

func (s *Service) CreateGroup(ctx context.Context, displayName string) error {
	_, err := s.identityClient.CreateGroup(ctx, &identitystore.CreateGroupInput{
		IdentityStoreId: &s.identityStoreID,
		DisplayName:     &displayName,
	})
	return err
}

func (s *Service) DeleteGroup(ctx context.Context, groupID string) error {
	_, err := s.identityClient.DeleteGroup(ctx, &identitystore.DeleteGroupInput{
		IdentityStoreId: &s.identityStoreID,
		GroupId:         &groupID,
	})
	return err
}

func (s *Service) GroupMembershipCount(ctx context.Context, groupID string) (int, error) {
	count := 0
	pager := identitystore.NewListGroupMembershipsPaginator(s.identityClient, &identitystore.ListGroupMembershipsInput{
		IdentityStoreId: &s.identityStoreID,
		GroupId:         &groupID,
	})

	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return 0, err
		}
		count += len(page.GroupMemberships)
	}

	return count, nil
}

func (s *Service) ListGroupUsers(ctx context.Context, groupID string) ([]GroupUser, error) {
	result := make([]GroupUser, 0, 64)
	pager := identitystore.NewListGroupMembershipsPaginator(s.identityClient, &identitystore.ListGroupMembershipsInput{
		IdentityStoreId: &s.identityStoreID,
		GroupId:         &groupID,
	})

	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, m := range page.GroupMemberships {
			userID := memberIDUserID(m.MemberId)
			member := GroupUser{
				MembershipID: value(m.MembershipId),
				UserID:       userID,
			}

			var detail *identitystore.DescribeUserOutput
			var err error
			if userID != "" {
				detail, err = s.identityClient.DescribeUser(ctx, &identitystore.DescribeUserInput{
					IdentityStoreId: &s.identityStoreID,
					UserId:          &userID,
				})
			}
			if err == nil {
				member.DisplayName = value(detail.DisplayName)
				member.Email = firstUserEmail(detail.Emails)
				if member.DisplayName == "" {
					member.DisplayName = value(detail.UserName)
				}
			}

			if member.DisplayName == "" {
				member.DisplayName = member.UserID
			}

			result = append(result, member)
		}
	}

	return result, nil
}

func (s *Service) ListUsers(ctx context.Context) ([]User, error) {
	users := make([]User, 0, 256)
	pager := identitystore.NewListUsersPaginator(s.identityClient, &identitystore.ListUsersInput{
		IdentityStoreId: &s.identityStoreID,
	})

	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, u := range page.Users {
			user := User{
				ID:          value(u.UserId),
				DisplayName: value(u.DisplayName),
				UserName:    value(u.UserName),
				Email:       firstUserEmail(u.Emails),
			}
			if user.DisplayName == "" {
				if user.UserName != "" {
					user.DisplayName = user.UserName
				} else {
					user.DisplayName = user.ID
				}
			}
			users = append(users, user)
		}
	}

	return users, nil
}

func (s *Service) AddUserToGroup(ctx context.Context, groupID, userID string) error {
	_, err := s.identityClient.CreateGroupMembership(ctx, &identitystore.CreateGroupMembershipInput{
		IdentityStoreId: &s.identityStoreID,
		GroupId:         &groupID,
		MemberId: &identitytypes.MemberIdMemberUserId{
			Value: userID,
		},
	})
	return err
}

func (s *Service) RemoveUserFromGroup(ctx context.Context, membershipID string) error {
	_, err := s.identityClient.DeleteGroupMembership(ctx, &identitystore.DeleteGroupMembershipInput{
		IdentityStoreId: &s.identityStoreID,
		MembershipId:    &membershipID,
	})
	return err
}

func (s *Service) ListAccounts(ctx context.Context) ([]Account, error) {
	accounts := make([]Account, 0, 256)
	pager := organizations.NewListAccountsPaginator(s.orgClient, &organizations.ListAccountsInput{})

	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			var accessDenied *orgtypes.AccessDeniedException
			if errors.As(err, &accessDenied) {
				return nil, ErrOrganizationsAccessDenied
			}

			if strings.Contains(strings.ToLower(err.Error()), "accessdenied") {
				return nil, ErrOrganizationsAccessDenied
			}

			return nil, err
		}

		for _, account := range page.Accounts {
			accounts = append(accounts, Account{
				ID:    value(account.Id),
				Name:  value(account.Name),
				Email: value(account.Email),
			})
		}
	}

	return accounts, nil
}

func (s *Service) ListPermissionSets(ctx context.Context) ([]PermissionSet, error) {
	arns := make([]string, 0, 128)
	pager := ssoadmin.NewListPermissionSetsPaginator(s.ssoAdminClient, &ssoadmin.ListPermissionSetsInput{
		InstanceArn: &s.instanceARN,
	})

	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		arns = append(arns, page.PermissionSets...)
	}

	sets := make([]PermissionSet, 0, len(arns))
	for _, arn := range arns {
		resp, err := s.ssoAdminClient.DescribePermissionSet(ctx, &ssoadmin.DescribePermissionSetInput{
			InstanceArn:      &s.instanceARN,
			PermissionSetArn: &arn,
		})
		if err != nil {
			return nil, err
		}

		name := arn
		if resp.PermissionSet != nil && resp.PermissionSet.Name != nil && *resp.PermissionSet.Name != "" {
			name = *resp.PermissionSet.Name
		}

		sets = append(sets, PermissionSet{ARN: arn, Name: name})
	}

	return sets, nil
}

func (s *Service) DiscoverAssignments(ctx context.Context, groupID string, accounts []Account, permissionSets []PermissionSet) ([]Assignment, error) {
	assignments := make([]Assignment, 0, 256)

	for _, account := range accounts {
		for _, ps := range permissionSets {
			select {
			case <-ctx.Done():
				return assignments, ctx.Err()
			default:
			}

			pager := ssoadmin.NewListAccountAssignmentsPaginator(s.ssoAdminClient, &ssoadmin.ListAccountAssignmentsInput{
				InstanceArn:      &s.instanceARN,
				AccountId:        &account.ID,
				PermissionSetArn: &ps.ARN,
			})

			for pager.HasMorePages() {
				page, err := pager.NextPage(ctx)
				if err != nil {
					return assignments, err
				}

				for _, a := range page.AccountAssignments {
					if a.PrincipalType == ssoadmintypes.PrincipalTypeGroup && value(a.PrincipalId) == groupID {
						assignments = append(assignments, Assignment{
							AccountID:         account.ID,
							AccountName:       account.Name,
							PermissionSetARN:  ps.ARN,
							PermissionSetName: ps.Name,
						})
					}
				}
			}
		}
	}

	return assignments, nil
}

func (s *Service) CreateAssignment(ctx context.Context, groupID, accountID, permissionSetARN string) error {
	resp, err := s.ssoAdminClient.CreateAccountAssignment(ctx, &ssoadmin.CreateAccountAssignmentInput{
		InstanceArn:      &s.instanceARN,
		PermissionSetArn: &permissionSetARN,
		PrincipalType:    ssoadmintypes.PrincipalTypeGroup,
		PrincipalId:      &groupID,
		TargetType:       ssoadmintypes.TargetTypeAwsAccount,
		TargetId:         &accountID,
	})
	if err != nil {
		return err
	}

	if resp.AccountAssignmentCreationStatus == nil || resp.AccountAssignmentCreationStatus.RequestId == nil {
		return fmt.Errorf("missing assignment creation request id")
	}

	requestID := *resp.AccountAssignmentCreationStatus.RequestId
	return s.pollCreation(ctx, requestID)
}

func (s *Service) DeleteAssignment(ctx context.Context, groupID, accountID, permissionSetARN string) error {
	resp, err := s.ssoAdminClient.DeleteAccountAssignment(ctx, &ssoadmin.DeleteAccountAssignmentInput{
		InstanceArn:      &s.instanceARN,
		PermissionSetArn: &permissionSetARN,
		PrincipalType:    ssoadmintypes.PrincipalTypeGroup,
		PrincipalId:      &groupID,
		TargetType:       ssoadmintypes.TargetTypeAwsAccount,
		TargetId:         &accountID,
	})
	if err != nil {
		return err
	}

	if resp.AccountAssignmentDeletionStatus == nil || resp.AccountAssignmentDeletionStatus.RequestId == nil {
		return fmt.Errorf("missing assignment deletion request id")
	}

	requestID := *resp.AccountAssignmentDeletionStatus.RequestId
	return s.pollDeletion(ctx, requestID)
}

func (s *Service) loadClients(ctx context.Context) error {
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(s.region),
		awsconfig.WithSharedConfigProfile(s.profile),
	)
	if err != nil {
		return err
	}

	s.identityClient = identitystore.NewFromConfig(cfg)
	s.ssoAdminClient = ssoadmin.NewFromConfig(cfg)
	s.orgClient = organizations.NewFromConfig(cfg)

	return nil
}

func (s *Service) listInstances(ctx context.Context) ([]Instance, error) {
	pager := ssoadmin.NewListInstancesPaginator(s.ssoAdminClient, &ssoadmin.ListInstancesInput{})
	instances := make([]Instance, 0, 4)

	for pager.HasMorePages() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, i := range page.Instances {
			instances = append(instances, Instance{
				ARN:            value(i.InstanceArn),
				IdentityStore:  value(i.IdentityStoreId),
				DisplayName:    instanceDisplayName(i),
				IdentitySource: value(i.OwnerAccountId),
			})
		}
	}

	return instances, nil
}

func (s *Service) loginSSO(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "aws", "sso", "login", "--profile", s.profile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (s *Service) pollCreation(ctx context.Context, requestID string) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}

		resp, err := s.ssoAdminClient.DescribeAccountAssignmentCreationStatus(ctx, &ssoadmin.DescribeAccountAssignmentCreationStatusInput{
			InstanceArn:                        &s.instanceARN,
			AccountAssignmentCreationRequestId: &requestID,
		})
		if err != nil {
			return err
		}

		if resp.AccountAssignmentCreationStatus == nil {
			continue
		}

		s := resp.AccountAssignmentCreationStatus.Status
		if s == ssoadmintypes.StatusValuesSucceeded {
			return nil
		}
		if s == ssoadmintypes.StatusValuesFailed {
			reason := value(resp.AccountAssignmentCreationStatus.FailureReason)
			if reason == "" {
				reason = "unknown failure"
			}
			return fmt.Errorf("assignment creation failed: %s", reason)
		}
	}
}

func (s *Service) pollDeletion(ctx context.Context, requestID string) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}

		resp, err := s.ssoAdminClient.DescribeAccountAssignmentDeletionStatus(ctx, &ssoadmin.DescribeAccountAssignmentDeletionStatusInput{
			InstanceArn:                        &s.instanceARN,
			AccountAssignmentDeletionRequestId: &requestID,
		})
		if err != nil {
			return err
		}

		if resp.AccountAssignmentDeletionStatus == nil {
			continue
		}

		s := resp.AccountAssignmentDeletionStatus.Status
		if s == ssoadmintypes.StatusValuesSucceeded {
			return nil
		}
		if s == ssoadmintypes.StatusValuesFailed {
			reason := value(resp.AccountAssignmentDeletionStatus.FailureReason)
			if reason == "" {
				reason = "unknown failure"
			}
			return fmt.Errorf("assignment deletion failed: %s", reason)
		}
	}
}

func isSSOAuthError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	signals := []string{
		"sso session",
		"token has expired",
		"expired token",
		"unauthorized",
		"invalid_grant",
	}

	for _, signal := range signals {
		if strings.Contains(msg, signal) {
			return true
		}
	}

	return false
}

func instanceDisplayName(instance ssoadmintypes.InstanceMetadata) string {
	if instance.InstanceArn == nil || *instance.InstanceArn == "" {
		return "instance"
	}
	parts := strings.Split(*instance.InstanceArn, "/")
	return parts[len(parts)-1]
}

func firstUserEmail(values []identitytypes.Email) string {
	if len(values) == 0 {
		return ""
	}
	return value(values[0].Value)
}

func memberIDUserID(memberID identitytypes.MemberId) string {
	if memberID == nil {
		return ""
	}

	switch v := memberID.(type) {
	case *identitytypes.MemberIdMemberUserId:
		return v.Value
	default:
		return ""
	}
}

func value(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}
