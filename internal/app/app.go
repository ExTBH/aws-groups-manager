package app

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	awsvc "aws-groups-manager/internal/aws"
	"aws-groups-manager/internal/theme"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type StartConfig struct {
	Profile string
	Region  string
}

type screen int

const (
	screenRegion screen = iota
	screenProfile
	screenEnsureSession
	screenInstance
	screenGroups
	screenGroupDetail
)

type detailTab int

const (
	tabUsers detailTab = iota
	tabAccounts
)

type statusLevel int

const (
	statusInfo statusLevel = iota
	statusWarn
	statusErr
)

type modalType int

const (
	modalNone modalType = iota
	modalHelp
	modalErrorDetails
	modalGroupCreateInput
	modalGroupDeleteConfirm
	modalUserRemoveConfirm
	modalUserPicker
	modalAssignmentRemoveConfirm
	modalAccountPicker
	modalManualAccountInput
	modalPermissionSetPicker
	modalAssignmentCreateConfirm
	modalBlockingError
)

type uiItem struct {
	id    string
	title string
	desc  string
	raw   any
}

func (i uiItem) Title() string       { return i.title }
func (i uiItem) Description() string { return i.desc }
func (i uiItem) FilterValue() string { return i.title + " " + i.desc }

type statusMessage struct {
	level statusLevel
	text  string
}

type blockingError struct {
	title   string
	message string
	details string
}

type model struct {
	styles theme.Styles
	spin   spinner.Model

	width  int
	height int

	startCfg StartConfig

	screen screen
	tab    detailTab
	list   list.Model

	modal     modalType
	modalList list.Model
	input     textinput.Model

	status      statusMessage
	lastErr     error
	lastDetails string
	blockErr    blockingError

	busy bool

	filterEnabled bool

	profile string
	region  string

	svc       *awsvc.Service
	instances []awsvc.Instance
	instance  awsvc.Instance

	groups      []awsvc.Group
	groupCounts map[string]int
	group       awsvc.Group

	users []awsvc.GroupUser

	accounts            []awsvc.Account
	permissionSets      []awsvc.PermissionSet
	assignments         []awsvc.Assignment
	organizationsDenied bool

	selectedAccount       awsvc.Account
	selectedManualAccount string
	selectedPermissionSet awsvc.PermissionSet
	pendingRemoveUser     awsvc.GroupUser
	pendingRemoveAssign   awsvc.Assignment

	discoverCancel context.CancelFunc
}

type itemDelegate struct {
	styles theme.Styles
}

func (d itemDelegate) Height() int  { return 2 }
func (d itemDelegate) Spacing() int { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd {
	return nil
}

func (d itemDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	i := item.(uiItem)
	selected := index == m.Index()
	marker := "  "
	title := d.styles.NormalTitle.Render(i.title)
	desc := d.styles.NormalSub.Render(i.desc)
	if selected {
		marker = "▸ "
		title = d.styles.SelectedTitle.Render(i.title)
		desc = d.styles.SelectedSub.Render(i.desc)
	}
	fmt.Fprintf(w, "%s%s\n  %s", marker, title, desc)
}

type ensureSessionMsg struct {
	svc       *awsvc.Service
	instances []awsvc.Instance
	err       error
}

type groupsMsg struct {
	groups []awsvc.Group
	err    error
}

type groupCountMsg struct {
	groupID string
	count   int
	err     error
}

type usersMsg struct {
	users []awsvc.GroupUser
	err   error
}

type allUsersMsg struct {
	users []awsvc.User
	err   error
}

type accountsDiscoveryMsg struct {
	accounts       []awsvc.Account
	permissionSets []awsvc.PermissionSet
	assignments    []awsvc.Assignment
	orgDenied      bool
	err            error
}

type mutationMsg struct {
	operation string
	err       error
}

func Run(cfg StartConfig, output io.Writer) error {
	m := newModel(cfg)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithOutput(output))
	_, err := p.Run()
	return err
}

func newModel(cfg StartConfig) model {
	styles := theme.Dark()
	sp := spinner.New()
	sp.Spinner = spinner.MiniDot

	m := model{
		styles:        styles,
		spin:          sp,
		startCfg:      cfg,
		profile:       cfg.Profile,
		region:        cfg.Region,
		status:        statusMessage{level: statusInfo, text: "Ready"},
		groupCounts:   make(map[string]int),
		filterEnabled: true,
	}

	m.list = newList(styles)
	m.modalList = newList(styles)
	m.input = textinput.New()
	m.input.Prompt = "> "
	m.input.CharLimit = 120

	if cfg.Region == "" {
		m.screen = screenRegion
		m.list.Title = "Select region"
		m.setListItems(regionsToItems())
	} else if cfg.Profile == "" {
		m.screen = screenProfile
		m.list.Title = "Select profile"
	} else {
		m.screen = screenEnsureSession
		m.busy = true
		m.status = statusMessage{level: statusInfo, text: "Checking SSO session"}
	}

	return m
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.spin.Tick}

	if m.screen == screenProfile {
		cmds = append(cmds, loadProfilesCmd())
	}

	if m.screen == screenEnsureSession {
		cmds = append(cmds, ensureSessionCmd(m.profile, m.region))
	}

	return tea.Batch(cmds...)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(max(20, msg.Width-2), max(8, msg.Height-8))
		m.modalList.SetSize(max(30, msg.Width-10), max(8, msg.Height/2))

	case spinner.TickMsg:
		if m.busy {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			cmds = append(cmds, cmd)
		}

	case profilesMsg:
		m.busy = false
		if msg.err != nil {
			m.setBlockingError("Failed loading profiles", msg.err, "Check ~/.aws/config and ~/.aws/credentials")
			break
		}
		m.setListItems(profilesToItems(msg.profiles))
		m.status = statusMessage{level: statusInfo, text: "Select an AWS profile"}

	case ensureSessionMsg:
		m.busy = false
		if msg.err != nil {
			m.setBlockingError("Unable to establish SSO session", msg.err, "Run `aws sso login --profile <profile>` and retry")
			break
		}

		m.svc = msg.svc
		m.instances = msg.instances
		if len(msg.instances) == 0 {
			m.setBlockingError("No Identity Center instances found", fmt.Errorf("ListInstances returned zero instances"), "Verify account/region and IAM Identity Center setup")
			break
		}

		if len(msg.instances) == 1 {
			m.instance = msg.instances[0]
			m.svc.SetInstance(m.instance.ARN, m.instance.IdentityStore)
			m.screen = screenGroups
			m.status = statusMessage{level: statusInfo, text: "Loaded Identity Center instance"}
			m.configureListForGroups()
			m.busy = true
			cmds = append(cmds, loadGroupsCmd(m.svc))
			break
		}

		m.screen = screenInstance
		m.list.Title = "Select Identity Center instance"
		m.setListItems(instancesToItems(msg.instances))
		m.status = statusMessage{level: statusInfo, text: "Select an instance"}

	case groupsMsg:
		m.busy = false
		if msg.err != nil {
			m.setStatusErr("Failed to load groups", msg.err)
			break
		}
		m.groups = msg.groups
		m.groupCounts = map[string]int{}
		m.setListItems(groupsToItems(msg.groups, "", 0))
		if len(m.groups) > 0 {
			m.group = m.groups[0]
			m.setListItems(groupsToItems(m.groups, m.group.ID, unknownCount))
			cmds = append(cmds, loadGroupCountCmd(m.svc, m.group.ID))
		}
		m.status = statusMessage{level: statusInfo, text: fmt.Sprintf("Loaded %d groups", len(m.groups))}

	case groupCountMsg:
		if msg.err != nil {
			m.setStatusErr("Failed loading group user count", msg.err)
			break
		}
		m.groupCounts[msg.groupID] = msg.count
		if m.screen == screenGroups {
			selected := m.currentGroupID()
			count := m.groupCounts[selected]
			m.setListItems(groupsToItems(m.groups, selected, count))
		}

	case usersMsg:
		m.busy = false
		if msg.err != nil {
			m.setStatusErr("Failed to load group users", msg.err)
			break
		}
		m.users = msg.users
		if m.screen == screenGroupDetail && m.tab == tabUsers {
			m.setListItems(groupUsersToItems(msg.users))
		}
		m.status = statusMessage{level: statusInfo, text: fmt.Sprintf("Loaded %d users", len(msg.users))}

	case allUsersMsg:
		m.busy = false
		if msg.err != nil {
			m.setStatusErr("Failed to load users", msg.err)
			break
		}
		m.modal = modalUserPicker
		m.modalList.Title = "Select user to add"
		m.modalList.SetItems(allUsersToItems(msg.users))
		m.status = statusMessage{level: statusInfo, text: "Choose a user and press Enter"}

	case accountsDiscoveryMsg:
		m.busy = false
		m.discoverCancel = nil
		if msg.err != nil {
			m.setStatusErr("Failed to load accounts/assignments", msg.err)
			break
		}
		m.accounts = msg.accounts
		m.permissionSets = msg.permissionSets
		m.assignments = msg.assignments
		m.organizationsDenied = msg.orgDenied
		if m.screen == screenGroupDetail && m.tab == tabAccounts {
			m.setListItems(assignmentsToItems(msg.assignments))
		}
		if m.organizationsDenied {
			m.status = statusMessage{level: statusWarn, text: "Organizations access denied; use manual account ID for new assignments"}
		} else {
			m.status = statusMessage{level: statusInfo, text: fmt.Sprintf("Loaded %d assignments", len(msg.assignments))}
		}

	case mutationMsg:
		m.busy = false
		if msg.err != nil {
			m.setStatusErr(msg.operation+" failed", msg.err)
			break
		}
		m.status = statusMessage{level: statusInfo, text: msg.operation + " complete"}
		m.modal = modalNone

		if m.screen == screenGroups {
			m.busy = true
			cmds = append(cmds, loadGroupsCmd(m.svc))
		}
		if m.screen == screenGroupDetail && m.tab == tabUsers {
			m.busy = true
			cmds = append(cmds, loadGroupUsersCmd(m.svc, m.group.ID))
		}
		if m.screen == screenGroupDetail && m.tab == tabAccounts {
			m.busy = true
			ctx, cancel := context.WithCancel(context.Background())
			m.discoverCancel = cancel
			cmds = append(cmds, discoverAccountsAssignmentsCmd(ctx, m.svc, m.group.ID))
		}

	case tea.KeyMsg:
		if m.modal != modalNone {
			if cmd := m.handleModalKeyMsg(msg); cmd != nil {
				cmds = append(cmds, cmd)
			}
		} else {
			key := msg.String()
			if cmd := m.handleKey(msg); cmd != nil {
				cmds = append(cmds, cmd)
			}

			if key != "ctrl+c" {
				cmds = append(cmds, m.updateMainList(msg)...)
			}
		}

	default:
		cmds = append(cmds, m.updateMainList(msg)...)
	}

	return m, tea.Batch(cmds...)
}

func (m *model) updateMainList(msg tea.Msg) []tea.Cmd {
	prevGroup := m.currentGroupID()
	prevIndex := m.list.Index()
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	cmds := []tea.Cmd{cmd}

	if m.screen == screenGroups && prevIndex != m.list.Index() {
		selected := m.currentGroupID()
		if selected != "" && selected != prevGroup {
			m.group = m.groups[m.list.Index()]
			if _, ok := m.groupCounts[selected]; !ok {
				m.setListItems(groupsToItems(m.groups, selected, unknownCount))
				cmds = append(cmds, loadGroupCountCmd(m.svc, selected))
			} else {
				m.setListItems(groupsToItems(m.groups, selected, m.groupCounts[selected]))
			}
		}
	}

	return cmds
}

func (m model) View() string {
	headerStyle := m.styles.Header
	bodyStyle := m.styles.Body
	statusBase := m.styles.StatusInfo
	footerStyle := m.styles.Footer

	if m.width > 2 {
		headerStyle = headerStyle.Width(m.width - 2)
		bodyStyle = bodyStyle.Width(m.width - 2)
		statusBase = statusBase.Width(m.width - 2)
		footerStyle = footerStyle.Width(m.width - 2)
	}

	header := headerStyle.Render(m.headerText())
	body := m.list.View()
	if m.busy {
		body = m.spin.View() + " " + body
	}

	if m.screen == screenGroupDetail {
		body = m.renderTabs() + "\n" + body
	}

	status := m.renderStatus(statusBase)
	footer := footerStyle.Render(m.footerText())

	content := lipgloss.JoinVertical(lipgloss.Left, header, bodyStyle.Render(body), status, footer)

	if m.modal != modalNone {
		return overlay(m.styles, content, m.renderModal())
	}

	return m.styles.App.Render(content)
}

func (m *model) handleKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()

	if key == "ctrl+c" {
		if m.discoverCancel != nil {
			m.discoverCancel()
		}
		return tea.Quit
	}

	switch key {
	case "ctrl+g":
		m.modal = modalHelp
		return nil
	case "ctrl+e":
		if m.lastErr != nil {
			m.modal = modalErrorDetails
		}
		return nil
	case "ctrl+f":
		m.filterEnabled = !m.filterEnabled
		m.list.SetFilteringEnabled(m.filterEnabled)
		m.status = statusMessage{level: statusInfo, text: "Toggled search/filter"}
		return nil
	case "ctrl+r":
		return m.refreshCurrentScreen()
	case "enter":
		return m.handleEnter()
	case "esc":
		return m.handleEsc()
	case "tab", "shift+tab":
		if m.screen == screenGroupDetail {
			if m.tab == tabUsers {
				m.tab = tabAccounts
				m.configureListForAssignments()
				m.setListItems(assignmentsToItems(m.assignments))
				if !m.busy {
					ctx, cancel := context.WithCancel(context.Background())
					m.discoverCancel = cancel
					m.busy = true
					return discoverAccountsAssignmentsCmd(ctx, m.svc, m.group.ID)
				}
			} else {
				m.tab = tabUsers
				m.configureListForUsers()
				m.setListItems(groupUsersToItems(m.users))
			}
		}
		return nil
	}

	if key == "ctrl+n" && m.screen == screenGroups {
		m.modal = modalGroupCreateInput
		m.input.SetValue("")
		m.input.Placeholder = "Group display name"
		m.input.Focus()
		return nil
	}

	if key == "ctrl+d" && m.screen == screenGroups {
		if m.currentGroupID() == "" {
			return nil
		}
		m.modal = modalGroupDeleteConfirm
		return nil
	}

	if m.screen == screenGroupDetail && m.tab == tabUsers {
		if key == "ctrl+a" {
			m.busy = true
			return loadAllUsersCmd(m.svc)
		}
		if key == "ctrl+x" {
			idx := m.list.Index()
			if idx >= 0 && idx < len(m.users) {
				m.pendingRemoveUser = m.users[idx]
				m.modal = modalUserRemoveConfirm
			}
			return nil
		}
	}

	if m.screen == screenGroupDetail && m.tab == tabAccounts {
		if key == "ctrl+a" {
			if len(m.permissionSets) == 0 {
				m.setStatusErr("Cannot add assignment", fmt.Errorf("permission sets are not loaded"))
				return nil
			}

			if m.organizationsDenied {
				m.modal = modalManualAccountInput
				m.input.SetValue("")
				m.input.Placeholder = "12-digit account ID"
				m.input.Focus()
				return nil
			}

			m.modal = modalAccountPicker
			m.modalList.Title = "Select account"
			m.modalList.SetItems(accountsToItems(m.accounts))
			return nil
		}

		if key == "ctrl+x" {
			idx := m.list.Index()
			if idx >= 0 && idx < len(m.assignments) {
				m.pendingRemoveAssign = m.assignments[idx]
				m.modal = modalAssignmentRemoveConfirm
			}
			return nil
		}
	}

	return nil
}

func (m *model) handleModalKeyMsg(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()

	if key == "esc" {
		if m.modal == modalBlockingError {
			return nil
		}
		m.modal = modalNone
		m.input.Blur()
		return nil
	}

	if key == "enter" {
		switch m.modal {
		case modalHelp, modalErrorDetails:
			m.modal = modalNone
		case modalBlockingError:
			m.modal = modalNone
		case modalGroupCreateInput:
			name := strings.TrimSpace(m.input.Value())
			if name == "" {
				m.status = statusMessage{level: statusWarn, text: "Group name cannot be empty"}
				return nil
			}
			m.modal = modalNone
			m.busy = true
			return createGroupCmd(m.svc, name)
		case modalGroupDeleteConfirm:
			groupID := m.currentGroupID()
			if groupID == "" {
				m.modal = modalNone
				return nil
			}
			m.modal = modalNone
			m.busy = true
			return deleteGroupCmd(m.svc, groupID)
		case modalUserRemoveConfirm:
			m.modal = modalNone
			m.busy = true
			return removeUserCmd(m.svc, m.pendingRemoveUser.MembershipID)
		case modalUserPicker:
			idx := m.modalList.Index()
			if idx < 0 || idx >= len(m.modalList.Items()) {
				return nil
			}
			item := m.modalList.Items()[idx].(uiItem)
			m.modal = modalNone
			m.busy = true
			return addUserCmd(m.svc, m.group.ID, item.id)
		case modalAccountPicker:
			idx := m.modalList.Index()
			if idx < 0 || idx >= len(m.accounts) {
				return nil
			}
			m.selectedAccount = m.accounts[idx]
			m.modal = modalPermissionSetPicker
			m.modalList.Title = "Select permission set"
			m.modalList.SetItems(permissionSetsToItems(m.permissionSets))
		case modalManualAccountInput:
			value := strings.TrimSpace(m.input.Value())
			if !isAccountID(value) {
				m.status = statusMessage{level: statusWarn, text: "Account ID must be 12 digits"}
				return nil
			}
			m.selectedManualAccount = value
			m.modal = modalPermissionSetPicker
			m.modalList.Title = "Select permission set"
			m.modalList.SetItems(permissionSetsToItems(m.permissionSets))
		case modalPermissionSetPicker:
			idx := m.modalList.Index()
			if idx < 0 || idx >= len(m.permissionSets) {
				return nil
			}
			m.selectedPermissionSet = m.permissionSets[idx]
			m.modal = modalAssignmentCreateConfirm
		case modalAssignmentCreateConfirm:
			accountID := m.selectedManualAccount
			if accountID == "" {
				accountID = m.selectedAccount.ID
			}
			if accountID == "" {
				m.status = statusMessage{level: statusWarn, text: "Select account first"}
				return nil
			}
			m.modal = modalNone
			m.busy = true
			return createAssignmentCmd(m.svc, m.group.ID, accountID, m.selectedPermissionSet.ARN)
		case modalAssignmentRemoveConfirm:
			m.modal = modalNone
			m.busy = true
			return deleteAssignmentCmd(m.svc, m.group.ID, m.pendingRemoveAssign.AccountID, m.pendingRemoveAssign.PermissionSetARN)
		}
	}

	if m.modalUsesList() {
		var cmd tea.Cmd
		m.modalList, cmd = m.modalList.Update(msg)
		return cmd
	}

	if m.modalUsesInput() {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return cmd
	}

	return nil
}

func (m *model) handleEnter() tea.Cmd {
	if m.busy {
		return nil
	}

	switch m.screen {
	case screenRegion:
		item := selectedItem(m.list)
		if item.id == "" {
			return nil
		}
		m.region = item.id
		if m.profile == "" {
			m.screen = screenProfile
			m.list.Title = "Select profile"
			m.setListItems(nil)
			m.busy = true
			return loadProfilesCmd()
		}
		m.screen = screenEnsureSession
		m.busy = true
		return ensureSessionCmd(m.profile, m.region)

	case screenProfile:
		item := selectedItem(m.list)
		if item.id == "" {
			return nil
		}
		m.profile = item.id
		m.screen = screenEnsureSession
		m.busy = true
		m.status = statusMessage{level: statusInfo, text: "Checking SSO session"}
		return ensureSessionCmd(m.profile, m.region)

	case screenInstance:
		idx := m.list.Index()
		if idx < 0 || idx >= len(m.instances) {
			return nil
		}
		m.instance = m.instances[idx]
		m.svc.SetInstance(m.instance.ARN, m.instance.IdentityStore)
		m.screen = screenGroups
		m.configureListForGroups()
		m.busy = true
		return loadGroupsCmd(m.svc)

	case screenGroups:
		idx := m.list.Index()
		if idx < 0 || idx >= len(m.groups) {
			return nil
		}
		m.group = m.groups[idx]
		m.screen = screenGroupDetail
		m.tab = tabUsers
		m.configureListForUsers()
		m.busy = true
		return loadGroupUsersCmd(m.svc, m.group.ID)
	}

	return nil
}

func (m *model) handleEsc() tea.Cmd {
	if m.busy && m.screen == screenGroupDetail && m.tab == tabAccounts && m.discoverCancel != nil {
		m.discoverCancel()
		m.discoverCancel = nil
		m.busy = false
		m.status = statusMessage{level: statusWarn, text: "Assignment discovery canceled"}
		return nil
	}

	if m.screen == screenGroupDetail {
		m.screen = screenGroups
		m.configureListForGroups()
		m.setListItems(groupsToItems(m.groups, m.group.ID, m.groupCounts[m.group.ID]))
		return nil
	}

	return nil
}

func (m *model) refreshCurrentScreen() tea.Cmd {
	if m.busy {
		return nil
	}

	switch m.screen {
	case screenGroups:
		m.busy = true
		return loadGroupsCmd(m.svc)
	case screenGroupDetail:
		if m.tab == tabUsers {
			m.busy = true
			return loadGroupUsersCmd(m.svc, m.group.ID)
		}
		ctx, cancel := context.WithCancel(context.Background())
		m.discoverCancel = cancel
		m.busy = true
		return discoverAccountsAssignmentsCmd(ctx, m.svc, m.group.ID)
	}

	return nil
}

func (m *model) setStatusErr(prefix string, err error) {
	m.lastErr = err
	m.lastDetails = fmt.Sprintf("%s\nerror: %v\nprofile: %s\nregion: %s\ninstance: %s", prefix, err, m.profile, m.region, m.instance.ARN)
	m.status = statusMessage{level: statusErr, text: fmt.Sprintf("%s: %v", prefix, err)}
}

func (m *model) setBlockingError(title string, err error, nextStep string) {
	m.lastErr = err
	m.lastDetails = fmt.Sprintf("%s\nerror: %v\nprofile: %s\nregion: %s", title, err, m.profile, m.region)
	m.blockErr = blockingError{
		title:   title,
		message: err.Error(),
		details: nextStep,
	}
	m.modal = modalBlockingError
	m.status = statusMessage{level: statusErr, text: title}
	m.busy = false
}

func (m model) headerText() string {
	instance := "-"
	if m.instance.ARN != "" {
		instance = shortARN(m.instance.ARN)
	}
	profile := m.profile
	if profile == "" {
		profile = "-"
	}
	region := m.region
	if region == "" {
		region = "-"
	}
	return fmt.Sprintf("aws-groups-manager | profile: %s | region: %s | instance: %s", profile, region, instance)
}

func (m model) renderStatus(base lipgloss.Style) string {
	text := m.status.text
	if m.busy {
		text = m.spin.View() + " " + text
	}

	switch m.status.level {
	case statusWarn:
		if m.width > 2 {
			return m.styles.StatusWarn.Width(m.width - 2).Render(text)
		}
		return m.styles.StatusWarn.Render(text)
	case statusErr:
		if m.width > 2 {
			return m.styles.StatusError.Width(m.width - 2).Render(text)
		}
		return m.styles.StatusError.Render(text)
	default:
		return base.Render(text)
	}
}

func (m model) renderTabs() string {
	users := m.styles.TabInactive.Render("Users")
	accounts := m.styles.TabInactive.Render("Accounts")
	if m.tab == tabUsers {
		users = m.styles.TabActive.Render("Users")
	} else {
		accounts = m.styles.TabActive.Render("Accounts")
	}
	return lipgloss.JoinHorizontal(lipgloss.Left, users, " ", accounts)
}

func (m model) footerText() string {
	items := []string{"^G Help", "^R Refresh", "^F Search"}

	if m.screen == screenGroups {
		items = append(items, "^N Create Group", "^D Delete Group")
	}

	if m.screen == screenGroupDetail {
		if m.tab == tabUsers {
			items = append(items, "^A Add User", "^X Remove User")
		} else {
			items = append(items, "^A Add Assignment", "^X Remove Assignment")
		}
	}

	items = append(items, "Enter Select", "Esc Back", "^C Quit")
	if m.lastErr != nil {
		items = append([]string{"^E Error"}, items...)
	}

	return strings.Join(items, "  ")
}

func (m model) renderModal() string {
	switch m.modal {
	case modalHelp:
		return m.styles.Modal.Render(
			m.styles.ModalTitle.Render("Help") + "\n\n" +
				"Navigation: arrows, Enter, Esc, Tab/Shift+Tab\n" +
				"Actions: Ctrl-only shortcuts shown in footer\n\n" +
				m.styles.ModalHint.Render("Enter/Esc to close"),
		)
	case modalErrorDetails:
		return m.styles.Modal.Render(
			m.styles.ModalTitle.Render("Error Details") + "\n\n" +
				m.lastDetails + "\n\n" +
				m.styles.ModalHint.Render("Enter/Esc to close"),
		)
	case modalBlockingError:
		return m.styles.Modal.Render(
			m.styles.ModalTitle.Render(m.blockErr.title) + "\n\n" +
				m.blockErr.message + "\n\n" +
				"Next step: " + m.blockErr.details + "\n\n" +
				m.styles.ModalHint.Render("Press Enter"),
		)
	case modalGroupCreateInput:
		return m.styles.Modal.Render(
			m.styles.ModalTitle.Render("Create Group") + "\n\n" +
				m.input.View() + "\n\n" +
				m.styles.ModalHint.Render("Enter create | Esc cancel"),
		)
	case modalGroupDeleteConfirm:
		groupName := "selected group"
		if idx := m.list.Index(); idx >= 0 && idx < len(m.groups) {
			groupName = m.groups[idx].DisplayName
		}
		return m.styles.Modal.Render(
			m.styles.ModalTitle.Render("Delete Group") + "\n\n" +
				fmt.Sprintf("Delete group %q?", groupName) + "\n\n" +
				m.styles.ModalHint.Render("Enter confirm | Esc cancel"),
		)
	case modalUserPicker, modalAccountPicker, modalPermissionSetPicker:
		return m.styles.Modal.Render(
			m.styles.ModalTitle.Render(m.modalList.Title) + "\n\n" +
				m.modalList.View() + "\n\n" +
				m.styles.ModalHint.Render("Enter select | Esc cancel"),
		)
	case modalUserRemoveConfirm:
		return m.styles.Modal.Render(
			m.styles.ModalTitle.Render("Remove User") + "\n\n" +
				fmt.Sprintf("Remove %q from this group?", m.pendingRemoveUser.DisplayName) + "\n\n" +
				m.styles.ModalHint.Render("Enter confirm | Esc cancel"),
		)
	case modalAssignmentRemoveConfirm:
		return m.styles.Modal.Render(
			m.styles.ModalTitle.Render("Remove Assignment") + "\n\n" +
				fmt.Sprintf("Remove %s on %s?", m.pendingRemoveAssign.PermissionSetName, m.pendingRemoveAssign.AccountID) + "\n\n" +
				m.styles.ModalHint.Render("Enter confirm | Esc cancel"),
		)
	case modalManualAccountInput:
		return m.styles.Modal.Render(
			m.styles.ModalTitle.Render("Manual Account ID") + "\n\n" +
				m.input.View() + "\n\n" +
				"Organizations access is unavailable. Enter account ID directly.\n\n" +
				m.styles.ModalHint.Render("Enter continue | Esc cancel"),
		)
	case modalAssignmentCreateConfirm:
		account := m.selectedManualAccount
		if account == "" {
			account = m.selectedAccount.ID
		}
		return m.styles.Modal.Render(
			m.styles.ModalTitle.Render("Create Assignment") + "\n\n" +
				fmt.Sprintf("Account: %s\nPermission set: %s", account, m.selectedPermissionSet.Name) + "\n\n" +
				m.styles.ModalHint.Render("Enter confirm | Esc cancel"),
		)
	}

	return ""
}

func (m model) modalUsesList() bool {
	return m.modal == modalUserPicker || m.modal == modalAccountPicker || m.modal == modalPermissionSetPicker
}

func (m model) modalUsesInput() bool {
	return m.modal == modalGroupCreateInput || m.modal == modalManualAccountInput
}

func (m *model) configureListForGroups() {
	m.list.Title = "Groups"
	m.list.ResetSelected()
	m.list.SetShowTitle(true)
}

func (m *model) configureListForUsers() {
	m.list.Title = "Group Users"
	m.list.ResetSelected()
	m.list.SetShowTitle(true)
}

func (m *model) configureListForAssignments() {
	m.list.Title = "Account Assignments"
	m.list.ResetSelected()
	m.list.SetShowTitle(true)
}

func (m *model) setListItems(items []list.Item) {
	currentIndex := m.list.Index()
	m.list.SetItems(items)
	if len(items) == 0 {
		return
	}
	if currentIndex < 0 || currentIndex >= len(items) {
		currentIndex = 0
	}
	m.list.Select(currentIndex)
}

func (m model) currentGroupID() string {
	idx := m.list.Index()
	if idx < 0 || idx >= len(m.groups) {
		return ""
	}
	return m.groups[idx].ID
}

type profilesMsg struct {
	profiles []string
	err      error
}

func loadProfilesCmd() tea.Cmd {
	return func() tea.Msg {
		profiles, err := loadProfiles()
		return profilesMsg{profiles: profiles, err: err}
	}
}

func ensureSessionCmd(profile, region string) tea.Cmd {
	return func() tea.Msg {
		svc := awsvc.NewService(profile, region)
		instances, err := svc.EnsureSession(context.Background())
		return ensureSessionMsg{svc: svc, instances: instances, err: err}
	}
}

func loadGroupsCmd(svc *awsvc.Service) tea.Cmd {
	return func() tea.Msg {
		groups, err := svc.ListGroups(context.Background())
		return groupsMsg{groups: groups, err: err}
	}
}

func loadGroupCountCmd(svc *awsvc.Service, groupID string) tea.Cmd {
	return func() tea.Msg {
		count, err := svc.GroupMembershipCount(context.Background(), groupID)
		return groupCountMsg{groupID: groupID, count: count, err: err}
	}
}

func createGroupCmd(svc *awsvc.Service, name string) tea.Cmd {
	return func() tea.Msg {
		err := svc.CreateGroup(context.Background(), name)
		return mutationMsg{operation: "Create group", err: err}
	}
}

func deleteGroupCmd(svc *awsvc.Service, groupID string) tea.Cmd {
	return func() tea.Msg {
		err := svc.DeleteGroup(context.Background(), groupID)
		return mutationMsg{operation: "Delete group", err: err}
	}
}

func loadGroupUsersCmd(svc *awsvc.Service, groupID string) tea.Cmd {
	return func() tea.Msg {
		users, err := svc.ListGroupUsers(context.Background(), groupID)
		return usersMsg{users: users, err: err}
	}
}

func loadAllUsersCmd(svc *awsvc.Service) tea.Cmd {
	return func() tea.Msg {
		users, err := svc.ListUsers(context.Background())
		return allUsersMsg{users: users, err: err}
	}
}

func addUserCmd(svc *awsvc.Service, groupID, userID string) tea.Cmd {
	return func() tea.Msg {
		err := svc.AddUserToGroup(context.Background(), groupID, userID)
		return mutationMsg{operation: "Add user", err: err}
	}
}

func removeUserCmd(svc *awsvc.Service, membershipID string) tea.Cmd {
	return func() tea.Msg {
		err := svc.RemoveUserFromGroup(context.Background(), membershipID)
		return mutationMsg{operation: "Remove user", err: err}
	}
}

func discoverAccountsAssignmentsCmd(ctx context.Context, svc *awsvc.Service, groupID string) tea.Cmd {
	return func() tea.Msg {
		accounts, err := svc.ListAccounts(ctx)
		orgDenied := false
		if err != nil {
			if err == awsvc.ErrOrganizationsAccessDenied {
				orgDenied = true
				accounts = nil
			} else {
				return accountsDiscoveryMsg{err: err}
			}
		}

		sets, err := svc.ListPermissionSets(ctx)
		if err != nil {
			return accountsDiscoveryMsg{err: err}
		}

		assignments := []awsvc.Assignment{}
		if !orgDenied {
			assignments, err = svc.DiscoverAssignments(ctx, groupID, accounts, sets)
			if err != nil && err != context.Canceled {
				return accountsDiscoveryMsg{err: err}
			}
		}

		return accountsDiscoveryMsg{
			accounts:       accounts,
			permissionSets: sets,
			assignments:    assignments,
			orgDenied:      orgDenied,
			err:            nil,
		}
	}
}

func createAssignmentCmd(svc *awsvc.Service, groupID, accountID, permissionSetARN string) tea.Cmd {
	return func() tea.Msg {
		err := svc.CreateAssignment(context.Background(), groupID, accountID, permissionSetARN)
		return mutationMsg{operation: "Create assignment", err: err}
	}
}

func deleteAssignmentCmd(svc *awsvc.Service, groupID, accountID, permissionSetARN string) tea.Cmd {
	return func() tea.Msg {
		err := svc.DeleteAssignment(context.Background(), groupID, accountID, permissionSetARN)
		return mutationMsg{operation: "Delete assignment", err: err}
	}
}

func newList(styles theme.Styles) list.Model {
	l := list.New([]list.Item{}, itemDelegate{styles: styles}, 0, 0)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.SetShowPagination(false)
	l.DisableQuitKeybindings()
	return l
}

func regionsToItems() []list.Item {
	items := make([]list.Item, 0, len(awsRegions))
	for _, region := range awsRegions {
		items = append(items, uiItem{id: region, title: region, desc: "AWS region"})
	}
	return items
}

func profilesToItems(profiles []string) []list.Item {
	items := make([]list.Item, 0, len(profiles))
	for _, profile := range profiles {
		items = append(items, uiItem{id: profile, title: profile, desc: "AWS profile"})
	}
	return items
}

const unknownCount = -1

func groupsToItems(groups []awsvc.Group, selectedID string, selectedCount int) []list.Item {
	items := make([]list.Item, 0, len(groups))
	for _, g := range groups {
		desc := "Users: -"
		if g.ID == selectedID {
			if selectedCount == unknownCount {
				desc = "Users: loading..."
			} else {
				desc = "Users: " + strconv.Itoa(selectedCount)
			}
		}
		items = append(items, uiItem{id: g.ID, title: g.DisplayName, desc: desc, raw: g})
	}
	return items
}

func instancesToItems(instances []awsvc.Instance) []list.Item {
	items := make([]list.Item, 0, len(instances))
	for _, instance := range instances {
		name := instance.DisplayName
		if name == "" {
			name = shortARN(instance.ARN)
		}
		items = append(items, uiItem{id: instance.ARN, title: name, desc: instance.IdentityStore, raw: instance})
	}
	return items
}

func groupUsersToItems(users []awsvc.GroupUser) []list.Item {
	items := make([]list.Item, 0, len(users))
	for _, user := range users {
		desc := user.Email
		if desc == "" {
			desc = user.UserID
		}
		items = append(items, uiItem{id: user.MembershipID, title: user.DisplayName, desc: desc, raw: user})
	}
	return items
}

func allUsersToItems(users []awsvc.User) []list.Item {
	items := make([]list.Item, 0, len(users))
	for _, user := range users {
		desc := user.Email
		if desc == "" {
			desc = user.UserName
		}
		items = append(items, uiItem{id: user.ID, title: user.DisplayName, desc: desc, raw: user})
	}
	return items
}

func accountsToItems(accounts []awsvc.Account) []list.Item {
	items := make([]list.Item, 0, len(accounts))
	for _, account := range accounts {
		items = append(items, uiItem{id: account.ID, title: account.Name, desc: account.ID, raw: account})
	}
	return items
}

func permissionSetsToItems(sets []awsvc.PermissionSet) []list.Item {
	items := make([]list.Item, 0, len(sets))
	for _, set := range sets {
		items = append(items, uiItem{id: set.ARN, title: set.Name, desc: set.ARN, raw: set})
	}
	return items
}

func assignmentsToItems(assignments []awsvc.Assignment) []list.Item {
	items := make([]list.Item, 0, len(assignments))
	for _, a := range assignments {
		title := fmt.Sprintf("%s (%s)", fallback(a.AccountName, a.AccountID), a.AccountID)
		items = append(items, uiItem{id: a.AccountID + "|" + a.PermissionSetARN, title: title, desc: a.PermissionSetName, raw: a})
	}
	return items
}

func selectedItem(l list.Model) uiItem {
	items := l.Items()
	idx := l.Index()
	if idx < 0 || idx >= len(items) {
		return uiItem{}
	}
	item, ok := items[idx].(uiItem)
	if !ok {
		return uiItem{}
	}
	return item
}

func shortARN(arn string) string {
	parts := strings.Split(arn, "/")
	return parts[len(parts)-1]
}

func fallback(value, fallbackValue string) string {
	if strings.TrimSpace(value) == "" {
		return fallbackValue
	}
	return value
}

func isAccountID(value string) bool {
	if len(value) != 12 {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func overlay(styles theme.Styles, content, modal string) string {
	if modal == "" {
		return content
	}
	return styles.App.Render(content + "\n\n" + modal)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
