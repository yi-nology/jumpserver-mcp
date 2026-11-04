package jmsms

// The composed answers: a tool's whole reply, put together from the
// vocabulary in the IDL.
//
// This file is hand-written — the files next to it are thriftgo's output
// (one per IDL domain), and gen-idl.sh only regenerates those. It exists
// because the IDL describes
// the WORDS (an asset, an account, a grant, a check), while a tool answers
// with several of them at once, and that combination is not something an IDL
// struct can say on its own.
//
// The rule it follows: a tool's answer is composed only out of shapes the
// IDL already defines. A tool that needs a new shape adds it to the IDL, not
// here.

import (
	"github.com/yi-nology/jumpserver-kit/model/jms"
)

// SecurityReviewResult is what jms_security_review answers.
//
// The diagnosis is the shared one; the payload below it is what the review
// looked at — who can reconfigure the bastion, whose password never moved,
// which addresses are hammering the login and which of them are already
// blocked.
type SecurityReviewResult struct {
	jms.Diagnosis

	// WindowDays is how many days of history were examined.
	WindowDays int32 `json:"window_days"`

	// Superusers are the accounts that can reconfigure the bastion.
	Superusers []string `json:"superusers"`

	// NeverLoggedIn are the accounts that have never signed in.
	NeverLoggedIn []string `json:"never_logged_in"`

	// PasswordNeverRotated are the accounts whose password never changed.
	PasswordNeverRotated []string `json:"password_never_rotated"`

	// DeactivatedUsers are the accounts that were switched off.
	DeactivatedUsers []string `json:"deactivated_users"`

	// FailedLogins is how many failed logins there were.
	FailedLogins int32 `json:"failed_logins"`

	// FailedByIP counts the failures per source address.
	FailedByIP map[string]int32 `json:"failed_by_ip"`

	// FailedByUser counts the failures per account.
	FailedByUser map[string]int32 `json:"failed_by_user"`

	// BlockedIPs are the addresses the bastion refuses.
	BlockedIPs []string `json:"blocked_ips"`

	// TopOffendingIPs are the addresses that failed the most.
	TopOffendingIPs []OffendingIP `json:"top_offending_ips"`
}

// CommandGuardResult is what jms_command_guard answers.
//
// The diagnosis is the shared one (the verdict sentence is in it); the
// payload says which filters were examined, which of them were in scope and
// which ones actually matched the command.
type CommandGuardResult struct {
	jms.Diagnosis

	// Command is the command that was checked.
	Command string `json:"command"`

	// MatchingFilters are the filters that matched.
	MatchingFilters []MatchedFilter `json:"matching_filters"`

	// FiltersExamined is how many filters exist.
	FiltersExamined int32 `json:"filters_examined"`

	// FiltersInScope is how many of them apply to this user and asset.
	FiltersInScope int32 `json:"filters_in_scope"`

	// Asset is the asset the command would run on, when one was named.
	Asset string `json:"asset,omitempty"`
}

// DoctorResult is what jms_doctor answers.
//
// The diagnosis is the shared one; the payload says what the check ran
// against: which bastion, as whom, and how many assets and permissions it
// looked at.
type DoctorResult struct {
	jms.Diagnosis

	// BaseURL is the bastion this server talks to.
	BaseURL string `json:"base_url"`

	// Identity is who the token acts as.
	Identity IdentityRef `json:"identity"`

	// AssetTotal is how many assets there are.
	AssetTotal int32 `json:"asset_total"`

	// AssetByPlatform counts the assets per platform.
	AssetByPlatform map[string]int32 `json:"asset_by_platform"`

	// PermissionTotal is how many permissions there are.
	PermissionTotal int32 `json:"permission_total"`

	// AllAssets lists every asset, when the check was asked to be verbose.
	AllAssets []jms.AssetSummary `json:"all_assets,omitempty"`
}

// LoginAssetCheckResult is what jms_login_asset_check answers.
//
// The diagnosis is the shared one; the payload says what was checked and
// what the bastion itself said when it was asked about a named user.
type LoginAssetCheckResult struct {
	jms.Diagnosis

	// Asset is the machine that was checked.
	Asset string `json:"asset"`

	// AssetID is its uuid.
	AssetID string `json:"asset_id"`

	// LoginACLs is how many login ACLs exist.
	LoginACLs int32 `json:"login_acls"`

	// LoginAssetACLs is how many login-asset ACLs exist.
	LoginAssetACLs int32 `json:"login_asset_acls"`

	// CheckedUser is the account the bastion was asked about.
	CheckedUser string `json:"checked_user,omitempty"`

	// UserLookupError says why the account could not be resolved.
	UserLookupError string `json:"user_lookup_error,omitempty"`

	// ServerCheckError says why the bastion's own check failed.
	ServerCheckError string `json:"server_check_error,omitempty"`

	// ServerCheck is what the bastion answered about the named account.
	ServerCheck *LoginCheckVerdict `json:"server_check,omitempty"`
}

// AuditAsset is the machine as jms_asset_audit describes it.
//
// It is the shared asset shape (jms.AssetSummary -- the same one the asset
// list and the execution plane's inventory hand out) plus the context that
// only this tool reports: where the machine sits in the asset tree, what it
// is tagged with, and when it was last verified.
type AuditAsset struct {
	jms.AssetSummary

	// NodesDisplay is the machine's place in the asset tree (the paths a
	// human reads).
	NodesDisplay []string `json:"nodes_display,omitempty"`

	// Labels are the tags on the machine.
	Labels []string `json:"labels,omitempty"`

	// Comment is the machine's note.
	Comment string `json:"comment,omitempty"`

	// Type is the bastion's own type word for it.
	Type string `json:"type,omitempty"`

	// DateVerified is when the connectivity was last checked.
	DateVerified string `json:"date_verified,omitempty"`

	// DateLastLogin is when somebody last logged into it.
	DateLastLogin string `json:"date_last_login,omitempty"`
}

// AssetAuditResult is what jms_asset_audit answers.
//
// It is the flagship read: one call that says what the machine is, who may
// log into it, and what is broken about it. The diagnosis part is the shared
// one (checks / findings / status / verdict / advice); the three lists are
// the machine itself and the two things that bear on it.
//
// The asset list rows are AccountSummary — the same shape jms_accounts_list
// hands out — so a caller that learned the vocabulary in one tool knows it in
// the other.
type AssetAuditResult struct {
	jms.Diagnosis

	// Asset is the machine: its platform, protocols, tree position, labels.
	Asset AuditAsset `json:"asset"`

	// Accounts are the logins this asset has.
	Accounts []AccountSummary `json:"accounts"`

	// Permissions are the grants that cover this asset, and what each of
	// them actually hands out.
	Permissions []PermissionGrant `json:"permissions"`
}

// AssetEvidenceResult is what jms_asset_evidence answers.
//
// It is the command-level trail of one machine: the sessions that carry audit
// value, and what was typed in them. The blind spots (sessions that had real
// interaction but left no command row) are reported on their own, because they
// are the part of the trail the command log cannot speak for.
type AssetEvidenceResult struct {
	// Asset is the machine the trail belongs to.
	Asset string `json:"asset"`

	// Address is where it is.
	Address string `json:"address,omitempty"`

	// Timeline holds the sessions that carry audit value, newest first.
	Timeline []EvidenceSession `json:"timeline"`

	// SessionsWithoutCommands holds the sessions that had real interaction
	// but left no command row.
	SessionsWithoutCommands []EvidenceSession `json:"sessions_without_commands,omitempty"`

	// SessionsOnAsset is how many sessions the machine has.
	SessionsOnAsset int `json:"sessions_on_asset"`

	// SessionsWithEvidence is how many of them carry audit value.
	SessionsWithEvidence int `json:"sessions_with_evidence"`

	// SessionsWithoutEvidence is how many were left out for carrying an
	// error_reason.
	SessionsWithoutEvidence int `json:"sessions_without_evidence,omitempty"`

	// CommandsReported is how many command rows the timeline carries.
	CommandsReported int `json:"commands_reported"`

	// ReplaysPulled is how many session recordings were pulled for the
	// sessions that had no command rows.
	ReplaysPulled int `json:"replays_pulled"`

	// AssetDeletedFromBastion says the machine is no longer registered and
	// the trail comes from the sessions it left behind.
	AssetDeletedFromBastion bool `json:"asset_deleted_from_bastion,omitempty"`

	// Note explains the scope of the answer.
	Note string `json:"note,omitempty"`

	// BlindSpotNote explains the blind spots.
	BlindSpotNote string `json:"blind_spot_note,omitempty"`
}

// AssetTouchHistoryResult is what jms_asset_touch_history answers.
//
// It is the per-asset view: who touched this machine, when, with which
// account, and how much of it is actually covered by the command log.
type AssetTouchHistoryResult struct {
	// Assets is the per-asset breakdown.
	Assets []TouchAsset `json:"assets"`

	// AssetsTouched is how many assets were touched at all.
	AssetsTouched int `json:"assets_touched"`

	// Sessions is how many sessions there were in total.
	Sessions int `json:"sessions"`

	// TopUsers is who touched the most.
	TopUsers []TouchTopUser `json:"top_users,omitempty"`

	// AssetsTouchedBySeveralUsers is the "is this machine too crowded" signal.
	AssetsTouchedBySeveralUsers []TouchMultiUser `json:"assets_touched_by_several_users,omitempty"`

	// Coverage is the audit-coverage breakdown.
	Coverage TouchCoverage `json:"coverage,omitempty"`

	// Since is the date filter that was applied.
	Since string `json:"since,omitempty"`

	// FilteredToAsset is the asset filter that was applied.
	FilteredToAsset string `json:"filtered_to_asset,omitempty"`

	// Note explains the coverage numbers.
	Note string `json:"note,omitempty"`
}
