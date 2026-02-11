# AWS Operation Map

## Bootstrap/Auth
- `ssoadmin.ListInstances` as auth/session gate.
- On expired/missing SSO session: run `aws sso login --profile <profile>`, then retry once.

## Groups Screen
- Load groups: `identitystore.ListGroups`
- Selected-group user count: `identitystore.ListGroupMemberships` count
- Create group: `identitystore.CreateGroup`
- Delete group: `identitystore.DeleteGroup`

## Group Detail - Users
- Memberships: `identitystore.ListGroupMemberships`
- User metadata: `identitystore.DescribeUser`
- Add user: `identitystore.ListUsers` -> `identitystore.CreateGroupMembership`
- Remove user: `identitystore.DeleteGroupMembership`

## Group Detail - Accounts
- Accounts: `organizations.ListAccounts` (if permitted)
- Permission sets: `ssoadmin.ListPermissionSets` + `DescribePermissionSet`
- Assignment discovery: `ssoadmin.ListAccountAssignments` per account x permission set
- Create assignment: `ssoadmin.CreateAccountAssignment` + poll `DescribeAccountAssignmentCreationStatus`
- Delete assignment: `ssoadmin.DeleteAccountAssignment` + poll `DescribeAccountAssignmentDeletionStatus`

## Polling Rules
- Poll every ~2 seconds until success/failure/cancel.
- Surface failure reason in status when available.

## Organizations Denied Behavior
- Accounts list discovery is skipped.
- Accounts tab remains accessible.
- Add assignment supports manual account ID entry.
