-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30min';

-- Phase 17 WP1 step 2 of 4: the checks prepared in 00025 are validated,
-- which lets SET NOT NULL skip its table scan; the checks are then dropped.
-- New rows take the connection's tenant (app_tenant_id(), set by the pool
-- from the request or job context): a connection without a tenant cannot
-- insert, and row-level security (00028) rejects any other value.

-- ai_models
ALTER TABLE ai_models VALIDATE CONSTRAINT ai_models_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE ai_models ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE ai_models DROP CONSTRAINT ai_models_tenant_id_not_null;
ALTER TABLE ai_models VALIDATE CONSTRAINT ai_models_tenant_id_fkey;
ALTER TABLE ai_models ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- ai_profiles
ALTER TABLE ai_profiles VALIDATE CONSTRAINT ai_profiles_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE ai_profiles ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE ai_profiles DROP CONSTRAINT ai_profiles_tenant_id_not_null;
ALTER TABLE ai_profiles VALIDATE CONSTRAINT ai_profiles_tenant_id_fkey;
ALTER TABLE ai_profiles ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- ai_providers
ALTER TABLE ai_providers VALIDATE CONSTRAINT ai_providers_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE ai_providers ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE ai_providers DROP CONSTRAINT ai_providers_tenant_id_not_null;
ALTER TABLE ai_providers VALIDATE CONSTRAINT ai_providers_tenant_id_fkey;
ALTER TABLE ai_providers ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- ai_usage
ALTER TABLE ai_usage VALIDATE CONSTRAINT ai_usage_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE ai_usage ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE ai_usage DROP CONSTRAINT ai_usage_tenant_id_not_null;
ALTER TABLE ai_usage VALIDATE CONSTRAINT ai_usage_tenant_id_fkey;
ALTER TABLE ai_usage ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- api_keys
ALTER TABLE api_keys VALIDATE CONSTRAINT api_keys_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE api_keys ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE api_keys DROP CONSTRAINT api_keys_tenant_id_not_null;
ALTER TABLE api_keys VALIDATE CONSTRAINT api_keys_tenant_id_fkey;
ALTER TABLE api_keys ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- audit_logs
ALTER TABLE audit_logs VALIDATE CONSTRAINT audit_logs_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE audit_logs ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE audit_logs DROP CONSTRAINT audit_logs_tenant_id_not_null;
ALTER TABLE audit_logs VALIDATE CONSTRAINT audit_logs_tenant_id_fkey;
ALTER TABLE audit_logs ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- automation_run_steps
ALTER TABLE automation_run_steps VALIDATE CONSTRAINT automation_run_steps_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE automation_run_steps ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE automation_run_steps DROP CONSTRAINT automation_run_steps_tenant_id_not_null;
ALTER TABLE automation_run_steps VALIDATE CONSTRAINT automation_run_steps_tenant_id_fkey;
ALTER TABLE automation_run_steps ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- automation_runs
ALTER TABLE automation_runs VALIDATE CONSTRAINT automation_runs_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE automation_runs ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE automation_runs DROP CONSTRAINT automation_runs_tenant_id_not_null;
ALTER TABLE automation_runs VALIDATE CONSTRAINT automation_runs_tenant_id_fkey;
ALTER TABLE automation_runs ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- automation_sends
ALTER TABLE automation_sends VALIDATE CONSTRAINT automation_sends_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE automation_sends ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE automation_sends DROP CONSTRAINT automation_sends_tenant_id_not_null;
ALTER TABLE automation_sends VALIDATE CONSTRAINT automation_sends_tenant_id_fkey;
ALTER TABLE automation_sends ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- automations
ALTER TABLE automations VALIDATE CONSTRAINT automations_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE automations ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE automations DROP CONSTRAINT automations_tenant_id_not_null;
ALTER TABLE automations VALIDATE CONSTRAINT automations_tenant_id_fkey;
ALTER TABLE automations ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- campaign_counters
ALTER TABLE campaign_counters VALIDATE CONSTRAINT campaign_counters_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE campaign_counters ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE campaign_counters DROP CONSTRAINT campaign_counters_tenant_id_not_null;
ALTER TABLE campaign_counters VALIDATE CONSTRAINT campaign_counters_tenant_id_fkey;
ALTER TABLE campaign_counters ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- campaign_events
ALTER TABLE campaign_events VALIDATE CONSTRAINT campaign_events_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE campaign_events ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE campaign_events DROP CONSTRAINT campaign_events_tenant_id_not_null;
ALTER TABLE campaign_events VALIDATE CONSTRAINT campaign_events_tenant_id_fkey;
ALTER TABLE campaign_events ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- campaign_exports
ALTER TABLE campaign_exports VALIDATE CONSTRAINT campaign_exports_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE campaign_exports ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE campaign_exports DROP CONSTRAINT campaign_exports_tenant_id_not_null;
ALTER TABLE campaign_exports VALIDATE CONSTRAINT campaign_exports_tenant_id_fkey;
ALTER TABLE campaign_exports ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- campaign_forecasts
ALTER TABLE campaign_forecasts VALIDATE CONSTRAINT campaign_forecasts_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE campaign_forecasts ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE campaign_forecasts DROP CONSTRAINT campaign_forecasts_tenant_id_not_null;
ALTER TABLE campaign_forecasts VALIDATE CONSTRAINT campaign_forecasts_tenant_id_fkey;
ALTER TABLE campaign_forecasts ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- campaign_pauses
ALTER TABLE campaign_pauses VALIDATE CONSTRAINT campaign_pauses_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE campaign_pauses ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE campaign_pauses DROP CONSTRAINT campaign_pauses_tenant_id_not_null;
ALTER TABLE campaign_pauses VALIDATE CONSTRAINT campaign_pauses_tenant_id_fkey;
ALTER TABLE campaign_pauses ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- campaign_recipients
ALTER TABLE campaign_recipients VALIDATE CONSTRAINT campaign_recipients_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE campaign_recipients ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE campaign_recipients DROP CONSTRAINT campaign_recipients_tenant_id_not_null;
ALTER TABLE campaign_recipients VALIDATE CONSTRAINT campaign_recipients_tenant_id_fkey;
ALTER TABLE campaign_recipients ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- campaign_reply_classifications
ALTER TABLE campaign_reply_classifications VALIDATE CONSTRAINT campaign_reply_classifications_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE campaign_reply_classifications ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE campaign_reply_classifications DROP CONSTRAINT campaign_reply_classifications_tenant_id_not_null;
ALTER TABLE campaign_reply_classifications VALIDATE CONSTRAINT campaign_reply_classifications_tenant_id_fkey;
ALTER TABLE campaign_reply_classifications ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- campaigns
ALTER TABLE campaigns VALIDATE CONSTRAINT campaigns_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE campaigns ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE campaigns DROP CONSTRAINT campaigns_tenant_id_not_null;
ALTER TABLE campaigns VALIDATE CONSTRAINT campaigns_tenant_id_fkey;
ALTER TABLE campaigns ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- consent_events
ALTER TABLE consent_events VALIDATE CONSTRAINT consent_events_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE consent_events ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE consent_events DROP CONSTRAINT consent_events_tenant_id_not_null;
ALTER TABLE consent_events VALIDATE CONSTRAINT consent_events_tenant_id_fkey;
ALTER TABLE consent_events ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- contact_field_sources
ALTER TABLE contact_field_sources VALIDATE CONSTRAINT contact_field_sources_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE contact_field_sources ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE contact_field_sources DROP CONSTRAINT contact_field_sources_tenant_id_not_null;
ALTER TABLE contact_field_sources VALIDATE CONSTRAINT contact_field_sources_tenant_id_fkey;
ALTER TABLE contact_field_sources ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- contact_group_members
ALTER TABLE contact_group_members VALIDATE CONSTRAINT contact_group_members_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE contact_group_members ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE contact_group_members DROP CONSTRAINT contact_group_members_tenant_id_not_null;
ALTER TABLE contact_group_members VALIDATE CONSTRAINT contact_group_members_tenant_id_fkey;
ALTER TABLE contact_group_members ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- contact_groups
ALTER TABLE contact_groups VALIDATE CONSTRAINT contact_groups_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE contact_groups ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE contact_groups DROP CONSTRAINT contact_groups_tenant_id_not_null;
ALTER TABLE contact_groups VALIDATE CONSTRAINT contact_groups_tenant_id_fkey;
ALTER TABLE contact_groups ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- contact_imports
ALTER TABLE contact_imports VALIDATE CONSTRAINT contact_imports_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE contact_imports ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE contact_imports DROP CONSTRAINT contact_imports_tenant_id_not_null;
ALTER TABLE contact_imports VALIDATE CONSTRAINT contact_imports_tenant_id_fkey;
ALTER TABLE contact_imports ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- contact_notes
ALTER TABLE contact_notes VALIDATE CONSTRAINT contact_notes_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE contact_notes ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE contact_notes DROP CONSTRAINT contact_notes_tenant_id_not_null;
ALTER TABLE contact_notes VALIDATE CONSTRAINT contact_notes_tenant_id_fkey;
ALTER TABLE contact_notes ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- contact_tags
ALTER TABLE contact_tags VALIDATE CONSTRAINT contact_tags_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE contact_tags ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE contact_tags DROP CONSTRAINT contact_tags_tenant_id_not_null;
ALTER TABLE contact_tags VALIDATE CONSTRAINT contact_tags_tenant_id_fkey;
ALTER TABLE contact_tags ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- contacts
ALTER TABLE contacts VALIDATE CONSTRAINT contacts_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE contacts ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE contacts DROP CONSTRAINT contacts_tenant_id_not_null;
ALTER TABLE contacts VALIDATE CONSTRAINT contacts_tenant_id_fkey;
ALTER TABLE contacts ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- conversation_activity
ALTER TABLE conversation_activity VALIDATE CONSTRAINT conversation_activity_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE conversation_activity ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE conversation_activity DROP CONSTRAINT conversation_activity_tenant_id_not_null;
ALTER TABLE conversation_activity VALIDATE CONSTRAINT conversation_activity_tenant_id_fkey;
ALTER TABLE conversation_activity ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- conversation_ai_drafts
ALTER TABLE conversation_ai_drafts VALIDATE CONSTRAINT conversation_ai_drafts_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE conversation_ai_drafts ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE conversation_ai_drafts DROP CONSTRAINT conversation_ai_drafts_tenant_id_not_null;
ALTER TABLE conversation_ai_drafts VALIDATE CONSTRAINT conversation_ai_drafts_tenant_id_fkey;
ALTER TABLE conversation_ai_drafts ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- conversation_events
ALTER TABLE conversation_events VALIDATE CONSTRAINT conversation_events_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE conversation_events ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE conversation_events DROP CONSTRAINT conversation_events_tenant_id_not_null;
ALTER TABLE conversation_events VALIDATE CONSTRAINT conversation_events_tenant_id_fkey;
ALTER TABLE conversation_events ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- conversation_handoffs
ALTER TABLE conversation_handoffs VALIDATE CONSTRAINT conversation_handoffs_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE conversation_handoffs ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE conversation_handoffs DROP CONSTRAINT conversation_handoffs_tenant_id_not_null;
ALTER TABLE conversation_handoffs VALIDATE CONSTRAINT conversation_handoffs_tenant_id_fkey;
ALTER TABLE conversation_handoffs ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- conversation_notes
ALTER TABLE conversation_notes VALIDATE CONSTRAINT conversation_notes_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE conversation_notes ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE conversation_notes DROP CONSTRAINT conversation_notes_tenant_id_not_null;
ALTER TABLE conversation_notes VALIDATE CONSTRAINT conversation_notes_tenant_id_fkey;
ALTER TABLE conversation_notes ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- conversations
ALTER TABLE conversations VALIDATE CONSTRAINT conversations_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE conversations ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE conversations DROP CONSTRAINT conversations_tenant_id_not_null;
ALTER TABLE conversations VALIDATE CONSTRAINT conversations_tenant_id_fkey;
ALTER TABLE conversations ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- idempotency_keys
ALTER TABLE idempotency_keys VALIDATE CONSTRAINT idempotency_keys_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE idempotency_keys ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE idempotency_keys DROP CONSTRAINT idempotency_keys_tenant_id_not_null;
ALTER TABLE idempotency_keys VALIDATE CONSTRAINT idempotency_keys_tenant_id_fkey;
ALTER TABLE idempotency_keys ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- media_files
ALTER TABLE media_files VALIDATE CONSTRAINT media_files_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE media_files ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE media_files DROP CONSTRAINT media_files_tenant_id_not_null;
ALTER TABLE media_files VALIDATE CONSTRAINT media_files_tenant_id_fkey;
ALTER TABLE media_files ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- message_templates
ALTER TABLE message_templates VALIDATE CONSTRAINT message_templates_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE message_templates ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE message_templates DROP CONSTRAINT message_templates_tenant_id_not_null;
ALTER TABLE message_templates VALIDATE CONSTRAINT message_templates_tenant_id_fkey;
ALTER TABLE message_templates ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- messages
ALTER TABLE messages VALIDATE CONSTRAINT messages_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE messages ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE messages DROP CONSTRAINT messages_tenant_id_not_null;
ALTER TABLE messages VALIDATE CONSTRAINT messages_tenant_id_fkey;
ALTER TABLE messages ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- meta_cost_daily
ALTER TABLE meta_cost_daily VALIDATE CONSTRAINT meta_cost_daily_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE meta_cost_daily ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE meta_cost_daily DROP CONSTRAINT meta_cost_daily_tenant_id_not_null;
ALTER TABLE meta_cost_daily VALIDATE CONSTRAINT meta_cost_daily_tenant_id_fkey;
ALTER TABLE meta_cost_daily ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- meta_user_preferences
ALTER TABLE meta_user_preferences VALIDATE CONSTRAINT meta_user_preferences_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE meta_user_preferences ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE meta_user_preferences DROP CONSTRAINT meta_user_preferences_tenant_id_not_null;
ALTER TABLE meta_user_preferences VALIDATE CONSTRAINT meta_user_preferences_tenant_id_fkey;
ALTER TABLE meta_user_preferences ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- offers
ALTER TABLE offers VALIDATE CONSTRAINT offers_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE offers ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE offers DROP CONSTRAINT offers_tenant_id_not_null;
ALTER TABLE offers VALIDATE CONSTRAINT offers_tenant_id_fkey;
ALTER TABLE offers ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- outgoing_webhook_events
ALTER TABLE outgoing_webhook_events VALIDATE CONSTRAINT outgoing_webhook_events_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE outgoing_webhook_events ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE outgoing_webhook_events DROP CONSTRAINT outgoing_webhook_events_tenant_id_not_null;
ALTER TABLE outgoing_webhook_events VALIDATE CONSTRAINT outgoing_webhook_events_tenant_id_fkey;
ALTER TABLE outgoing_webhook_events ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- password_reset_tokens
ALTER TABLE password_reset_tokens VALIDATE CONSTRAINT password_reset_tokens_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE password_reset_tokens ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE password_reset_tokens DROP CONSTRAINT password_reset_tokens_tenant_id_not_null;
ALTER TABLE password_reset_tokens VALIDATE CONSTRAINT password_reset_tokens_tenant_id_fkey;
ALTER TABLE password_reset_tokens ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- phone_numbers
ALTER TABLE phone_numbers VALIDATE CONSTRAINT phone_numbers_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE phone_numbers ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE phone_numbers DROP CONSTRAINT phone_numbers_tenant_id_not_null;
ALTER TABLE phone_numbers VALIDATE CONSTRAINT phone_numbers_tenant_id_fkey;
ALTER TABLE phone_numbers ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- portfolio_reach
ALTER TABLE portfolio_reach VALIDATE CONSTRAINT portfolio_reach_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE portfolio_reach ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE portfolio_reach DROP CONSTRAINT portfolio_reach_tenant_id_not_null;
ALTER TABLE portfolio_reach VALIDATE CONSTRAINT portfolio_reach_tenant_id_fkey;
ALTER TABLE portfolio_reach ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- segments
ALTER TABLE segments VALIDATE CONSTRAINT segments_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE segments ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE segments DROP CONSTRAINT segments_tenant_id_not_null;
ALTER TABLE segments VALIDATE CONSTRAINT segments_tenant_id_fkey;
ALTER TABLE segments ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- settings
ALTER TABLE settings VALIDATE CONSTRAINT settings_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE settings ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE settings DROP CONSTRAINT settings_tenant_id_not_null;
ALTER TABLE settings VALIDATE CONSTRAINT settings_tenant_id_fkey;
ALTER TABLE settings ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- tags
ALTER TABLE tags VALIDATE CONSTRAINT tags_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE tags ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE tags DROP CONSTRAINT tags_tenant_id_not_null;
ALTER TABLE tags VALIDATE CONSTRAINT tags_tenant_id_fkey;
ALTER TABLE tags ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- user_invitations
ALTER TABLE user_invitations VALIDATE CONSTRAINT user_invitations_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE user_invitations ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE user_invitations DROP CONSTRAINT user_invitations_tenant_id_not_null;
ALTER TABLE user_invitations VALIDATE CONSTRAINT user_invitations_tenant_id_fkey;
ALTER TABLE user_invitations ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- user_recovery_codes
ALTER TABLE user_recovery_codes VALIDATE CONSTRAINT user_recovery_codes_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE user_recovery_codes ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE user_recovery_codes DROP CONSTRAINT user_recovery_codes_tenant_id_not_null;
ALTER TABLE user_recovery_codes VALIDATE CONSTRAINT user_recovery_codes_tenant_id_fkey;
ALTER TABLE user_recovery_codes ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- user_roles
ALTER TABLE user_roles VALIDATE CONSTRAINT user_roles_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE user_roles ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE user_roles DROP CONSTRAINT user_roles_tenant_id_not_null;
ALTER TABLE user_roles VALIDATE CONSTRAINT user_roles_tenant_id_fkey;
ALTER TABLE user_roles ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- user_sessions
ALTER TABLE user_sessions VALIDATE CONSTRAINT user_sessions_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE user_sessions ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE user_sessions DROP CONSTRAINT user_sessions_tenant_id_not_null;
ALTER TABLE user_sessions VALIDATE CONSTRAINT user_sessions_tenant_id_fkey;
ALTER TABLE user_sessions ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- users
ALTER TABLE users VALIDATE CONSTRAINT users_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE users ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE users DROP CONSTRAINT users_tenant_id_not_null;
ALTER TABLE users VALIDATE CONSTRAINT users_tenant_id_fkey;
ALTER TABLE users ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- webhook_deliveries
ALTER TABLE webhook_deliveries VALIDATE CONSTRAINT webhook_deliveries_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE webhook_deliveries ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE webhook_deliveries DROP CONSTRAINT webhook_deliveries_tenant_id_not_null;
ALTER TABLE webhook_deliveries VALIDATE CONSTRAINT webhook_deliveries_tenant_id_fkey;
ALTER TABLE webhook_deliveries ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- webhook_endpoints
ALTER TABLE webhook_endpoints VALIDATE CONSTRAINT webhook_endpoints_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE webhook_endpoints ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE webhook_endpoints DROP CONSTRAINT webhook_endpoints_tenant_id_not_null;
ALTER TABLE webhook_endpoints VALIDATE CONSTRAINT webhook_endpoints_tenant_id_fkey;
ALTER TABLE webhook_endpoints ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- webhook_events
ALTER TABLE webhook_events VALIDATE CONSTRAINT webhook_events_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE webhook_events ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE webhook_events DROP CONSTRAINT webhook_events_tenant_id_not_null;
ALTER TABLE webhook_events VALIDATE CONSTRAINT webhook_events_tenant_id_fkey;
ALTER TABLE webhook_events ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- whatsapp_account_events
ALTER TABLE whatsapp_account_events VALIDATE CONSTRAINT whatsapp_account_events_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE whatsapp_account_events ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE whatsapp_account_events DROP CONSTRAINT whatsapp_account_events_tenant_id_not_null;
ALTER TABLE whatsapp_account_events VALIDATE CONSTRAINT whatsapp_account_events_tenant_id_fkey;
ALTER TABLE whatsapp_account_events ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- whatsapp_accounts
ALTER TABLE whatsapp_accounts VALIDATE CONSTRAINT whatsapp_accounts_tenant_id_not_null;
-- The validated check above makes this a catalogue-only change.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE whatsapp_accounts ALTER COLUMN tenant_id SET NOT NULL;
-- squawk-ignore ban-drop-constraint
ALTER TABLE whatsapp_accounts DROP CONSTRAINT whatsapp_accounts_tenant_id_not_null;
ALTER TABLE whatsapp_accounts VALIDATE CONSTRAINT whatsapp_accounts_tenant_id_fkey;
ALTER TABLE whatsapp_accounts ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

-- outbox_events
ALTER TABLE outbox_events VALIDATE CONSTRAINT outbox_events_tenant_id_fkey;
ALTER TABLE outbox_events ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();

ALTER TABLE phone_numbers VALIDATE CONSTRAINT phone_numbers_account_id_fkey;
ALTER TABLE message_templates VALIDATE CONSTRAINT message_templates_account_id_fkey;
ALTER TABLE conversations VALIDATE CONSTRAINT conversations_contact_id_fkey;
ALTER TABLE conversations VALIDATE CONSTRAINT conversations_phone_number_id_fkey;
ALTER TABLE messages VALIDATE CONSTRAINT messages_contact_id_fkey;
ALTER TABLE messages VALIDATE CONSTRAINT messages_phone_number_id_fkey;
ALTER TABLE messages VALIDATE CONSTRAINT messages_campaign_id_fkey;
ALTER TABLE campaigns VALIDATE CONSTRAINT campaigns_phone_number_id_fkey;
ALTER TABLE campaign_recipients VALIDATE CONSTRAINT campaign_recipients_campaign_id_fkey;
ALTER TABLE campaign_recipients VALIDATE CONSTRAINT campaign_recipients_contact_id_fkey;
ALTER TABLE automation_runs VALIDATE CONSTRAINT automation_runs_automation_id_fkey;
ALTER TABLE automation_runs VALIDATE CONSTRAINT automation_runs_contact_id_fkey;
ALTER TABLE contact_tags VALIDATE CONSTRAINT contact_tags_contact_id_fkey;
ALTER TABLE contact_tags VALIDATE CONSTRAINT contact_tags_tag_id_fkey;
ALTER TABLE contact_group_members VALIDATE CONSTRAINT contact_group_members_group_id_fkey;
ALTER TABLE contact_group_members VALIDATE CONSTRAINT contact_group_members_contact_id_fkey;
ALTER TABLE conversation_notes VALIDATE CONSTRAINT conversation_notes_conversation_id_fkey;
ALTER TABLE user_roles VALIDATE CONSTRAINT user_roles_user_id_fkey;
ALTER TABLE user_sessions VALIDATE CONSTRAINT user_sessions_user_id_fkey;
ALTER TABLE webhook_deliveries VALIDATE CONSTRAINT webhook_deliveries_endpoint_id_fkey;
ALTER TABLE webhook_deliveries VALIDATE CONSTRAINT webhook_deliveries_event_id_fkey;
ALTER TABLE media_files VALIDATE CONSTRAINT media_files_message_id_fkey;
ALTER TABLE ai_models VALIDATE CONSTRAINT ai_models_provider_id_fkey;
-- +goose Down
-- squawk-ignore ban-drop-default
ALTER TABLE outbox_events ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-default
ALTER TABLE whatsapp_accounts ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE whatsapp_accounts ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE whatsapp_accounts ADD CONSTRAINT whatsapp_accounts_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE whatsapp_account_events ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE whatsapp_account_events ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE whatsapp_account_events ADD CONSTRAINT whatsapp_account_events_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE webhook_events ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE webhook_events ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE webhook_events ADD CONSTRAINT webhook_events_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE webhook_endpoints ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE webhook_endpoints ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE webhook_endpoints ADD CONSTRAINT webhook_endpoints_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE webhook_deliveries ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE webhook_deliveries ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE webhook_deliveries ADD CONSTRAINT webhook_deliveries_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE users ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE users ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE users ADD CONSTRAINT users_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE user_sessions ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE user_sessions ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE user_sessions ADD CONSTRAINT user_sessions_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE user_roles ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE user_roles ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE user_roles ADD CONSTRAINT user_roles_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE user_recovery_codes ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE user_recovery_codes ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE user_recovery_codes ADD CONSTRAINT user_recovery_codes_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE user_invitations ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE user_invitations ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE user_invitations ADD CONSTRAINT user_invitations_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE tags ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE tags ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE tags ADD CONSTRAINT tags_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE settings ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE settings ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE settings ADD CONSTRAINT settings_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE segments ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE segments ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE segments ADD CONSTRAINT segments_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE portfolio_reach ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE portfolio_reach ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE portfolio_reach ADD CONSTRAINT portfolio_reach_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE phone_numbers ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE phone_numbers ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE phone_numbers ADD CONSTRAINT phone_numbers_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE password_reset_tokens ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE password_reset_tokens ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE password_reset_tokens ADD CONSTRAINT password_reset_tokens_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE outgoing_webhook_events ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE outgoing_webhook_events ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE outgoing_webhook_events ADD CONSTRAINT outgoing_webhook_events_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE offers ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE offers ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE offers ADD CONSTRAINT offers_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE meta_user_preferences ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE meta_user_preferences ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE meta_user_preferences ADD CONSTRAINT meta_user_preferences_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE meta_cost_daily ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE meta_cost_daily ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE meta_cost_daily ADD CONSTRAINT meta_cost_daily_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE messages ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE messages ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE messages ADD CONSTRAINT messages_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE message_templates ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE message_templates ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE message_templates ADD CONSTRAINT message_templates_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE media_files ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE media_files ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE media_files ADD CONSTRAINT media_files_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE idempotency_keys ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE idempotency_keys ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE idempotency_keys ADD CONSTRAINT idempotency_keys_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE conversations ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE conversations ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE conversations ADD CONSTRAINT conversations_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE conversation_notes ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE conversation_notes ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE conversation_notes ADD CONSTRAINT conversation_notes_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE conversation_handoffs ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE conversation_handoffs ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE conversation_handoffs ADD CONSTRAINT conversation_handoffs_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE conversation_events ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE conversation_events ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE conversation_events ADD CONSTRAINT conversation_events_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE conversation_ai_drafts ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE conversation_ai_drafts ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE conversation_ai_drafts ADD CONSTRAINT conversation_ai_drafts_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE conversation_activity ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE conversation_activity ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE conversation_activity ADD CONSTRAINT conversation_activity_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE contacts ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE contacts ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE contacts ADD CONSTRAINT contacts_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE contact_tags ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE contact_tags ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE contact_tags ADD CONSTRAINT contact_tags_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE contact_notes ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE contact_notes ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE contact_notes ADD CONSTRAINT contact_notes_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE contact_imports ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE contact_imports ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE contact_imports ADD CONSTRAINT contact_imports_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE contact_groups ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE contact_groups ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE contact_groups ADD CONSTRAINT contact_groups_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE contact_group_members ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE contact_group_members ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE contact_group_members ADD CONSTRAINT contact_group_members_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE contact_field_sources ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE contact_field_sources ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE contact_field_sources ADD CONSTRAINT contact_field_sources_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE consent_events ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE consent_events ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE consent_events ADD CONSTRAINT consent_events_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE campaigns ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE campaigns ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE campaigns ADD CONSTRAINT campaigns_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE campaign_reply_classifications ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE campaign_reply_classifications ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE campaign_reply_classifications ADD CONSTRAINT campaign_reply_classifications_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE campaign_recipients ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE campaign_recipients ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE campaign_recipients ADD CONSTRAINT campaign_recipients_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE campaign_pauses ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE campaign_pauses ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE campaign_pauses ADD CONSTRAINT campaign_pauses_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE campaign_forecasts ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE campaign_forecasts ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE campaign_forecasts ADD CONSTRAINT campaign_forecasts_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE campaign_exports ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE campaign_exports ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE campaign_exports ADD CONSTRAINT campaign_exports_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE campaign_events ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE campaign_events ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE campaign_events ADD CONSTRAINT campaign_events_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE campaign_counters ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE campaign_counters ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE campaign_counters ADD CONSTRAINT campaign_counters_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE automations ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE automations ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE automations ADD CONSTRAINT automations_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE automation_sends ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE automation_sends ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE automation_sends ADD CONSTRAINT automation_sends_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE automation_runs ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE automation_runs ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE automation_runs ADD CONSTRAINT automation_runs_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE automation_run_steps ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE automation_run_steps ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE automation_run_steps ADD CONSTRAINT automation_run_steps_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE audit_logs ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE audit_logs ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE audit_logs ADD CONSTRAINT audit_logs_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE api_keys ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE api_keys ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE api_keys ADD CONSTRAINT api_keys_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE ai_usage ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE ai_usage ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE ai_usage ADD CONSTRAINT ai_usage_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE ai_providers ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE ai_providers ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE ai_providers ADD CONSTRAINT ai_providers_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE ai_profiles ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE ai_profiles ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE ai_profiles ADD CONSTRAINT ai_profiles_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
-- squawk-ignore ban-drop-default
ALTER TABLE ai_models ALTER COLUMN tenant_id DROP DEFAULT;
-- squawk-ignore ban-drop-not-null
ALTER TABLE ai_models ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE ai_models ADD CONSTRAINT ai_models_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
