// Package policy holds the tool catalogue's risk side.
//
// For every tool: what it does to the bastion, whether it can be undone, and
// what it says when it is called without a confirmation.
//
// This used to live in the handlers, spread over 28 places that each repeated
// "if !args.Confirm { refuse }" with its own wording. One table is one thing
// to keep correct — and a test can check it against the registry, which is
// what pkg/reg asks the handler layer to register.
//
// It is a package of its own rather than part of biz/router because the
// handler tests need it too: a registry without the policy would treat every
// tool as an unclassified write and refuse the read-only ones.
package policy

import (
	"github.com/yi-nology/jumpserver-kit/model/jms"
	"github.com/yi-nology/jumpserver-kit/pkg/reg"
)

// Policy is the risk side of the tool catalogue: what each tool does to the
// bastion, and what it says when it is called without a confirmation.
//
// The confirm sentences are the tools' own wording, byte for byte what each
// tool used to answer when it checked the flag itself. They are what the
// model reads, so they are part of the contract, not an implementation
// detail.
// Table is the tool catalogue's risk side.
var Table = reg.PolicyTable{
	"jms_access_matrix":  {Risk: jms.Risk_read},
	"jms_account_create": {Risk: jms.Risk_write, Confirm: "set confirm=true to create the account on the bastion"},
	"jms_account_delete": {Risk: jms.Risk_destructive, Confirm: "set confirm=true; this deletes the account"},
	"jms_account_update": {Risk: jms.Risk_write, Confirm: "set confirm=true to change the account"},
	"jms_accounts_list":  {Risk: jms.Risk_read},
	"jms_account_automation_create": {Risk: jms.Risk_write,
		Confirm: "set confirm=true; this creates an automation plan on the bastion"},
	"jms_account_automation_executions": {Risk: jms.Risk_read},
	"jms_account_automation_records":    {Risk: jms.Risk_read},
	"jms_account_automation_run": {Risk: jms.Risk_write,
		Confirm: "set confirm=true; this runs the plan against real machines"},
	"jms_account_automations_list": {Risk: jms.Risk_read},
	"jms_asset_audit":              {Risk: jms.Risk_read},
	"jms_asset_create":             {Risk: jms.Risk_write, Confirm: "set confirm=true to create the asset"},
	"jms_asset_delete":             {Risk: jms.Risk_destructive, Confirm: "set confirm=true to delete the asset"},
	"jms_asset_evidence":           {Risk: jms.Risk_read},
	"jms_asset_get":                {Risk: jms.Risk_read},
	"jms_asset_move":               {Risk: jms.Risk_write, Confirm: "set confirm=true to change the asset tree"},
	"jms_asset_touch_history":      {Risk: jms.Risk_read},
	"jms_asset_update":             {Risk: jms.Risk_write, Confirm: "set confirm=true to change the asset"},
	"jms_assets_bulk_create":       {Risk: jms.Risk_write, Confirm: "set confirm=true to create the assets"},
	"jms_assets_list":              {Risk: jms.Risk_read},
	"jms_clipboard_acl_create":     {Risk: jms.Risk_write, Confirm: "set confirm=true to create the clipboard rule"},
	"jms_clipboard_acl_delete":     {Risk: jms.Risk_destructive, Confirm: "set confirm=true; this deletes the clipboard rule"},
	"jms_clipboard_acl_update":     {Risk: jms.Risk_write, Confirm: "set confirm=true to change the clipboard rule"},
	"jms_clipboard_acls":           {Risk: jms.Risk_read},
	"jms_cmdfilter_create":         {Risk: jms.Risk_write, Confirm: "set confirm=true to create the command filter"},
	"jms_cmdfilter_delete":         {Risk: jms.Risk_destructive, Confirm: "set confirm=true; removing a filter reopens the commands it blocked"},
	"jms_cmdfilter_list":           {Risk: jms.Risk_read},
	"jms_cmdgroup_create":          {Risk: jms.Risk_write, Confirm: "set confirm=true to create the command group"},
	"jms_cmdgroup_delete":          {Risk: jms.Risk_destructive, Confirm: "set confirm=true; removing a group loosens the filters that reference it"},
	"jms_cmdgroup_list":            {Risk: jms.Risk_read},
	"jms_command_audit":            {Risk: jms.Risk_read},
	"jms_command_guard":            {Risk: jms.Risk_read},
	"jms_commands_search":          {Risk: jms.Risk_read},
	"jms_connectivity_check":       {Risk: jms.Risk_read},
	"jms_data_masking_rule_create": {Risk: jms.Risk_write, Confirm: "set confirm=true to create the masking rule"},
	"jms_data_masking_rule_delete": {Risk: jms.Risk_destructive, Confirm: "set confirm=true; this deletes the masking rule"},
	"jms_data_masking_rule_update": {Risk: jms.Risk_write, Confirm: "set confirm=true to change the masking rule"},
	"jms_data_masking_rules":       {Risk: jms.Risk_read},
	"jms_delegation_call": {Risk: jms.Risk_write, Conditional: true,
		Hint: "confirm=true when the method changes something (POST/PATCH/PUT/DELETE); a GET does not need it"},
	"jms_delegation_probe":  {Risk: jms.Risk_read},
	"jms_delegation_status": {Risk: jms.Risk_read},
	"jms_doctor":            {Risk: jms.Risk_read},
	"jms_file_transfers":    {Risk: jms.Risk_read},
	"jms_gather_apply":      {Risk: jms.Risk_write, Confirm: "set confirm=true; this creates accounts on the asset"},
	"jms_gather_run":        {Risk: jms.Risk_write, Confirm: "set confirm=true: this executes remote automation on the asset"},
	"jms_gathered_accounts": {Risk: jms.Risk_read},
	"jms_job_audit":         {Risk: jms.Risk_read},
	"jms_job_list":          {Risk: jms.Risk_read},
	"jms_job_run": {Risk: jms.Risk_write, Conditional: true,
		Hint: "confirm=true only when the command is classified DANGEROUS"},
	"jms_job_status":              {Risk: jms.Risk_read},
	"jms_job_stop":                {Risk: jms.Risk_destructive, Confirm: "set confirm=true to stop the execution; it cuts work off on every target"},
	"jms_label_attach":            {Risk: jms.Risk_write, Confirm: "set confirm=true to tag the assets"},
	"jms_label_create":            {Risk: jms.Risk_write, Confirm: "set confirm=true to create the label"},
	"jms_label_delete":            {Risk: jms.Risk_destructive, Confirm: "set confirm=true to delete the label"},
	"jms_label_resources":         {Risk: jms.Risk_read},
	"jms_labels_list":             {Risk: jms.Risk_read},
	"jms_login_acl_create":        {Risk: jms.Risk_write, Confirm: "set confirm=true; this rule decides who may log in to the bastion"},
	"jms_login_acl_delete":        {Risk: jms.Risk_destructive, Confirm: "set confirm=true; this deletes the login rule"},
	"jms_login_acl_update":        {Risk: jms.Risk_write, Confirm: "set confirm=true; this changes who may log in to the bastion"},
	"jms_login_asset_check":       {Risk: jms.Risk_read},
	"jms_login_logs":              {Risk: jms.Risk_read},
	"jms_nodes_create":            {Risk: jms.Risk_write, Confirm: "set confirm=true to create the node"},
	"jms_nodes_list":              {Risk: jms.Risk_read},
	"jms_net_gateways_list":       {Risk: jms.Risk_read},
	"jms_net_path":                {Risk: jms.Risk_read},
	"jms_net_zones_list":          {Risk: jms.Risk_read},
	"jms_operate_logs":            {Risk: jms.Risk_read},
	"jms_password_change_logs":    {Risk: jms.Risk_read},
	"jms_permission_compare":      {Risk: jms.Risk_read},
	"jms_permission_coverage":     {Risk: jms.Risk_read},
	"jms_permission_create":       {Risk: jms.Risk_write, Confirm: "set confirm=true to create the permission"},
	"jms_permission_delete":       {Risk: jms.Risk_destructive, Confirm: "set confirm=true to delete the permission"},
	"jms_permission_get":          {Risk: jms.Risk_read},
	"jms_permission_grant":        {Risk: jms.Risk_write, Confirm: "set confirm=true to change the permission"},
	"jms_permissions_list":        {Risk: jms.Risk_read},
	"jms_rbac_permissions":        {Risk: jms.Risk_read},
	"jms_rbac_role_matrix":        {Risk: jms.Risk_read},
	"jms_platforms_list":          {Risk: jms.Risk_read},
	"jms_role_permissions":        {Risk: jms.Risk_read},
	"jms_roles_list":              {Risk: jms.Risk_read},
	"jms_security_review":         {Risk: jms.Risk_read},
	"jms_session_replay":          {Risk: jms.Risk_read},
	"jms_session_terminate":       {Risk: jms.Risk_destructive, Confirm: "set confirm=true to terminate the session"},
	"jms_session_transcript":      {Risk: jms.Risk_read},
	"jms_sessions_list":           {Risk: jms.Risk_read},
	"jms_sessions_live":           {Risk: jms.Risk_read},
	"jms_status":                  {Risk: jms.Risk_read},
	"jms_system_role_permissions": {Risk: jms.Risk_read},
	"jms_terminals_list":          {Risk: jms.Risk_read},
	"jms_user_access":             {Risk: jms.Risk_read},
	"jms_user_create":             {Risk: jms.Risk_write, Confirm: "set confirm=true to create the user"},
	"jms_user_groups_list":        {Risk: jms.Risk_read},
	"jms_user_password_set":       {Risk: jms.Risk_write, Confirm: "set confirm=true to change the password"},
	"jms_user_set_active":         {Risk: jms.Risk_write, Confirm: "set confirm=true to change the user's active state"},
	"jms_user_sshkey_add":         {Risk: jms.Risk_write, Confirm: "set confirm=true to write the key to the bastion"},
	"jms_user_sshkey_remove":      {Risk: jms.Risk_destructive, Confirm: "set confirm=true to remove the key"},
	"jms_users_list":              {Risk: jms.Risk_read},
}
