-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30min';

-- Phase 17 WP1 step 1 of 4: every tenant-owned table gets tenant_id.
-- The column is added empty, every existing row is moved to tenant #1 (the
-- whole backfill is one transaction, so a failed migration leaves nothing
-- half done), and NOT NULL is prepared with a NOT VALID check that 00026
-- validates. Child tables reference their parents by (tenant_id, id), so a
-- row can never point at another tenant's row.

-- ai_models
ALTER TABLE ai_models ADD COLUMN tenant_id uuid;
UPDATE ai_models SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE ai_models ADD CONSTRAINT ai_models_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE ai_models ADD CONSTRAINT ai_models_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- ai_profiles
ALTER TABLE ai_profiles ADD COLUMN tenant_id uuid;
UPDATE ai_profiles SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE ai_profiles ADD CONSTRAINT ai_profiles_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE ai_profiles ADD CONSTRAINT ai_profiles_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- ai_providers
ALTER TABLE ai_providers ADD COLUMN tenant_id uuid;
UPDATE ai_providers SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE ai_providers ADD CONSTRAINT ai_providers_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE ai_providers ADD CONSTRAINT ai_providers_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- ai_usage
ALTER TABLE ai_usage ADD COLUMN tenant_id uuid;
UPDATE ai_usage SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE ai_usage ADD CONSTRAINT ai_usage_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE ai_usage ADD CONSTRAINT ai_usage_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- api_keys
ALTER TABLE api_keys ADD COLUMN tenant_id uuid;
UPDATE api_keys SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE api_keys ADD CONSTRAINT api_keys_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE api_keys ADD CONSTRAINT api_keys_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- audit_logs
ALTER TABLE audit_logs ADD COLUMN tenant_id uuid;
ALTER TABLE audit_logs DISABLE TRIGGER audit_logs_no_update;
UPDATE audit_logs SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE audit_logs ENABLE TRIGGER audit_logs_no_update;
ALTER TABLE audit_logs ADD CONSTRAINT audit_logs_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE audit_logs ADD CONSTRAINT audit_logs_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- automation_run_steps
ALTER TABLE automation_run_steps ADD COLUMN tenant_id uuid;
UPDATE automation_run_steps SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE automation_run_steps ADD CONSTRAINT automation_run_steps_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE automation_run_steps ADD CONSTRAINT automation_run_steps_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- automation_runs
ALTER TABLE automation_runs ADD COLUMN tenant_id uuid;
UPDATE automation_runs SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE automation_runs ADD CONSTRAINT automation_runs_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE automation_runs ADD CONSTRAINT automation_runs_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- automation_sends
ALTER TABLE automation_sends ADD COLUMN tenant_id uuid;
UPDATE automation_sends SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE automation_sends ADD CONSTRAINT automation_sends_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE automation_sends ADD CONSTRAINT automation_sends_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- automations
ALTER TABLE automations ADD COLUMN tenant_id uuid;
UPDATE automations SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE automations ADD CONSTRAINT automations_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE automations ADD CONSTRAINT automations_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- campaign_counters
ALTER TABLE campaign_counters ADD COLUMN tenant_id uuid;
UPDATE campaign_counters SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE campaign_counters ADD CONSTRAINT campaign_counters_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE campaign_counters ADD CONSTRAINT campaign_counters_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- campaign_events
ALTER TABLE campaign_events ADD COLUMN tenant_id uuid;
UPDATE campaign_events SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE campaign_events ADD CONSTRAINT campaign_events_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE campaign_events ADD CONSTRAINT campaign_events_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- campaign_exports
ALTER TABLE campaign_exports ADD COLUMN tenant_id uuid;
UPDATE campaign_exports SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE campaign_exports ADD CONSTRAINT campaign_exports_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE campaign_exports ADD CONSTRAINT campaign_exports_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- campaign_forecasts
ALTER TABLE campaign_forecasts ADD COLUMN tenant_id uuid;
ALTER TABLE campaign_forecasts DISABLE TRIGGER campaign_forecasts_no_update;
UPDATE campaign_forecasts SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE campaign_forecasts ENABLE TRIGGER campaign_forecasts_no_update;
ALTER TABLE campaign_forecasts ADD CONSTRAINT campaign_forecasts_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE campaign_forecasts ADD CONSTRAINT campaign_forecasts_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- campaign_pauses
ALTER TABLE campaign_pauses ADD COLUMN tenant_id uuid;
UPDATE campaign_pauses SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE campaign_pauses ADD CONSTRAINT campaign_pauses_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE campaign_pauses ADD CONSTRAINT campaign_pauses_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- campaign_recipients
ALTER TABLE campaign_recipients ADD COLUMN tenant_id uuid;
UPDATE campaign_recipients SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE campaign_recipients ADD CONSTRAINT campaign_recipients_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE campaign_recipients ADD CONSTRAINT campaign_recipients_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- campaign_reply_classifications
ALTER TABLE campaign_reply_classifications ADD COLUMN tenant_id uuid;
UPDATE campaign_reply_classifications SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE campaign_reply_classifications ADD CONSTRAINT campaign_reply_classifications_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE campaign_reply_classifications ADD CONSTRAINT campaign_reply_classifications_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- campaigns
ALTER TABLE campaigns ADD COLUMN tenant_id uuid;
UPDATE campaigns SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE campaigns ADD CONSTRAINT campaigns_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE campaigns ADD CONSTRAINT campaigns_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- consent_events
ALTER TABLE consent_events ADD COLUMN tenant_id uuid;
UPDATE consent_events SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE consent_events ADD CONSTRAINT consent_events_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE consent_events ADD CONSTRAINT consent_events_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- contact_field_sources
ALTER TABLE contact_field_sources ADD COLUMN tenant_id uuid;
UPDATE contact_field_sources SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE contact_field_sources ADD CONSTRAINT contact_field_sources_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE contact_field_sources ADD CONSTRAINT contact_field_sources_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- contact_group_members
ALTER TABLE contact_group_members ADD COLUMN tenant_id uuid;
UPDATE contact_group_members SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE contact_group_members ADD CONSTRAINT contact_group_members_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE contact_group_members ADD CONSTRAINT contact_group_members_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- contact_groups
ALTER TABLE contact_groups ADD COLUMN tenant_id uuid;
UPDATE contact_groups SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE contact_groups ADD CONSTRAINT contact_groups_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE contact_groups ADD CONSTRAINT contact_groups_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- contact_imports
ALTER TABLE contact_imports ADD COLUMN tenant_id uuid;
UPDATE contact_imports SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE contact_imports ADD CONSTRAINT contact_imports_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE contact_imports ADD CONSTRAINT contact_imports_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- contact_notes
ALTER TABLE contact_notes ADD COLUMN tenant_id uuid;
UPDATE contact_notes SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE contact_notes ADD CONSTRAINT contact_notes_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE contact_notes ADD CONSTRAINT contact_notes_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- contact_tags
ALTER TABLE contact_tags ADD COLUMN tenant_id uuid;
UPDATE contact_tags SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE contact_tags ADD CONSTRAINT contact_tags_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE contact_tags ADD CONSTRAINT contact_tags_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- contacts
ALTER TABLE contacts ADD COLUMN tenant_id uuid;
UPDATE contacts SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE contacts ADD CONSTRAINT contacts_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE contacts ADD CONSTRAINT contacts_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- conversation_activity
ALTER TABLE conversation_activity ADD COLUMN tenant_id uuid;
UPDATE conversation_activity SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE conversation_activity ADD CONSTRAINT conversation_activity_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE conversation_activity ADD CONSTRAINT conversation_activity_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- conversation_ai_drafts
ALTER TABLE conversation_ai_drafts ADD COLUMN tenant_id uuid;
UPDATE conversation_ai_drafts SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE conversation_ai_drafts ADD CONSTRAINT conversation_ai_drafts_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE conversation_ai_drafts ADD CONSTRAINT conversation_ai_drafts_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- conversation_events
ALTER TABLE conversation_events ADD COLUMN tenant_id uuid;
UPDATE conversation_events SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE conversation_events ADD CONSTRAINT conversation_events_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE conversation_events ADD CONSTRAINT conversation_events_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- conversation_handoffs
ALTER TABLE conversation_handoffs ADD COLUMN tenant_id uuid;
UPDATE conversation_handoffs SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE conversation_handoffs ADD CONSTRAINT conversation_handoffs_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE conversation_handoffs ADD CONSTRAINT conversation_handoffs_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- conversation_notes
ALTER TABLE conversation_notes ADD COLUMN tenant_id uuid;
UPDATE conversation_notes SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE conversation_notes ADD CONSTRAINT conversation_notes_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE conversation_notes ADD CONSTRAINT conversation_notes_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- conversations
ALTER TABLE conversations ADD COLUMN tenant_id uuid;
UPDATE conversations SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE conversations ADD CONSTRAINT conversations_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE conversations ADD CONSTRAINT conversations_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- idempotency_keys
ALTER TABLE idempotency_keys ADD COLUMN tenant_id uuid;
UPDATE idempotency_keys SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE idempotency_keys ADD CONSTRAINT idempotency_keys_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE idempotency_keys ADD CONSTRAINT idempotency_keys_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- media_files
ALTER TABLE media_files ADD COLUMN tenant_id uuid;
UPDATE media_files SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE media_files ADD CONSTRAINT media_files_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE media_files ADD CONSTRAINT media_files_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- message_templates
ALTER TABLE message_templates ADD COLUMN tenant_id uuid;
UPDATE message_templates SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE message_templates ADD CONSTRAINT message_templates_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE message_templates ADD CONSTRAINT message_templates_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- messages
ALTER TABLE messages ADD COLUMN tenant_id uuid;
UPDATE messages SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE messages ADD CONSTRAINT messages_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE messages ADD CONSTRAINT messages_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- meta_cost_daily
ALTER TABLE meta_cost_daily ADD COLUMN tenant_id uuid;
UPDATE meta_cost_daily SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE meta_cost_daily ADD CONSTRAINT meta_cost_daily_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE meta_cost_daily ADD CONSTRAINT meta_cost_daily_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- meta_user_preferences
ALTER TABLE meta_user_preferences ADD COLUMN tenant_id uuid;
UPDATE meta_user_preferences SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE meta_user_preferences ADD CONSTRAINT meta_user_preferences_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE meta_user_preferences ADD CONSTRAINT meta_user_preferences_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- offers
ALTER TABLE offers ADD COLUMN tenant_id uuid;
UPDATE offers SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE offers ADD CONSTRAINT offers_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE offers ADD CONSTRAINT offers_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- outgoing_webhook_events
ALTER TABLE outgoing_webhook_events ADD COLUMN tenant_id uuid;
UPDATE outgoing_webhook_events SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE outgoing_webhook_events ADD CONSTRAINT outgoing_webhook_events_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE outgoing_webhook_events ADD CONSTRAINT outgoing_webhook_events_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- password_reset_tokens
ALTER TABLE password_reset_tokens ADD COLUMN tenant_id uuid;
UPDATE password_reset_tokens SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE password_reset_tokens ADD CONSTRAINT password_reset_tokens_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE password_reset_tokens ADD CONSTRAINT password_reset_tokens_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- phone_numbers
ALTER TABLE phone_numbers ADD COLUMN tenant_id uuid;
UPDATE phone_numbers SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE phone_numbers ADD CONSTRAINT phone_numbers_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE phone_numbers ADD CONSTRAINT phone_numbers_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- portfolio_reach
ALTER TABLE portfolio_reach ADD COLUMN tenant_id uuid;
UPDATE portfolio_reach SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE portfolio_reach ADD CONSTRAINT portfolio_reach_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE portfolio_reach ADD CONSTRAINT portfolio_reach_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- segments
ALTER TABLE segments ADD COLUMN tenant_id uuid;
UPDATE segments SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE segments ADD CONSTRAINT segments_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE segments ADD CONSTRAINT segments_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- settings
ALTER TABLE settings ADD COLUMN tenant_id uuid;
UPDATE settings SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE settings ADD CONSTRAINT settings_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE settings ADD CONSTRAINT settings_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- tags
ALTER TABLE tags ADD COLUMN tenant_id uuid;
UPDATE tags SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE tags ADD CONSTRAINT tags_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE tags ADD CONSTRAINT tags_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- user_invitations
ALTER TABLE user_invitations ADD COLUMN tenant_id uuid;
UPDATE user_invitations SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE user_invitations ADD CONSTRAINT user_invitations_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE user_invitations ADD CONSTRAINT user_invitations_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- user_recovery_codes
ALTER TABLE user_recovery_codes ADD COLUMN tenant_id uuid;
UPDATE user_recovery_codes SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE user_recovery_codes ADD CONSTRAINT user_recovery_codes_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE user_recovery_codes ADD CONSTRAINT user_recovery_codes_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- user_roles
ALTER TABLE user_roles ADD COLUMN tenant_id uuid;
UPDATE user_roles SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE user_roles ADD CONSTRAINT user_roles_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE user_roles ADD CONSTRAINT user_roles_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- user_sessions
ALTER TABLE user_sessions ADD COLUMN tenant_id uuid;
UPDATE user_sessions SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE user_sessions ADD CONSTRAINT user_sessions_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE user_sessions ADD CONSTRAINT user_sessions_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- users
ALTER TABLE users ADD COLUMN tenant_id uuid;
UPDATE users SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE users ADD CONSTRAINT users_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE users ADD CONSTRAINT users_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- webhook_deliveries
ALTER TABLE webhook_deliveries ADD COLUMN tenant_id uuid;
UPDATE webhook_deliveries SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE webhook_deliveries ADD CONSTRAINT webhook_deliveries_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE webhook_deliveries ADD CONSTRAINT webhook_deliveries_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- webhook_endpoints
ALTER TABLE webhook_endpoints ADD COLUMN tenant_id uuid;
UPDATE webhook_endpoints SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE webhook_endpoints ADD CONSTRAINT webhook_endpoints_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE webhook_endpoints ADD CONSTRAINT webhook_endpoints_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- webhook_events
ALTER TABLE webhook_events ADD COLUMN tenant_id uuid;
UPDATE webhook_events SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE webhook_events ADD CONSTRAINT webhook_events_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE webhook_events ADD CONSTRAINT webhook_events_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- whatsapp_account_events
ALTER TABLE whatsapp_account_events ADD COLUMN tenant_id uuid;
UPDATE whatsapp_account_events SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE whatsapp_account_events ADD CONSTRAINT whatsapp_account_events_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE whatsapp_account_events ADD CONSTRAINT whatsapp_account_events_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- whatsapp_accounts
ALTER TABLE whatsapp_accounts ADD COLUMN tenant_id uuid;
UPDATE whatsapp_accounts SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE whatsapp_accounts ADD CONSTRAINT whatsapp_accounts_tenant_id_not_null CHECK (tenant_id IS NOT NULL) NOT VALID;
ALTER TABLE whatsapp_accounts ADD CONSTRAINT whatsapp_accounts_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- outbox_events
ALTER TABLE outbox_events ADD COLUMN tenant_id uuid;
UPDATE outbox_events SET tenant_id = '00000000-0000-0000-0000-000000000001' WHERE tenant_id IS NULL;
ALTER TABLE outbox_events ADD CONSTRAINT outbox_events_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;

-- Parents get a (tenant_id, id) key for the composite foreign keys.
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX ai_providers_tenant_id_id_key ON ai_providers (tenant_id, id);
-- squawk-ignore disallowed-unique-constraint
ALTER TABLE ai_providers ADD CONSTRAINT ai_providers_tenant_id_id_key UNIQUE USING INDEX ai_providers_tenant_id_id_key;
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX automations_tenant_id_id_key ON automations (tenant_id, id);
-- squawk-ignore disallowed-unique-constraint
ALTER TABLE automations ADD CONSTRAINT automations_tenant_id_id_key UNIQUE USING INDEX automations_tenant_id_id_key;
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX campaigns_tenant_id_id_key ON campaigns (tenant_id, id);
-- squawk-ignore disallowed-unique-constraint
ALTER TABLE campaigns ADD CONSTRAINT campaigns_tenant_id_id_key UNIQUE USING INDEX campaigns_tenant_id_id_key;
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX contact_groups_tenant_id_id_key ON contact_groups (tenant_id, id);
-- squawk-ignore disallowed-unique-constraint
ALTER TABLE contact_groups ADD CONSTRAINT contact_groups_tenant_id_id_key UNIQUE USING INDEX contact_groups_tenant_id_id_key;
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX contacts_tenant_id_id_key ON contacts (tenant_id, id);
-- squawk-ignore disallowed-unique-constraint
ALTER TABLE contacts ADD CONSTRAINT contacts_tenant_id_id_key UNIQUE USING INDEX contacts_tenant_id_id_key;
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX conversations_tenant_id_id_key ON conversations (tenant_id, id);
-- squawk-ignore disallowed-unique-constraint
ALTER TABLE conversations ADD CONSTRAINT conversations_tenant_id_id_key UNIQUE USING INDEX conversations_tenant_id_id_key;
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX messages_tenant_id_id_key ON messages (tenant_id, id);
-- squawk-ignore disallowed-unique-constraint
ALTER TABLE messages ADD CONSTRAINT messages_tenant_id_id_key UNIQUE USING INDEX messages_tenant_id_id_key;
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX outgoing_webhook_events_tenant_id_id_key ON outgoing_webhook_events (tenant_id, id);
-- squawk-ignore disallowed-unique-constraint
ALTER TABLE outgoing_webhook_events ADD CONSTRAINT outgoing_webhook_events_tenant_id_id_key UNIQUE USING INDEX outgoing_webhook_events_tenant_id_id_key;
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX phone_numbers_tenant_id_id_key ON phone_numbers (tenant_id, id);
-- squawk-ignore disallowed-unique-constraint
ALTER TABLE phone_numbers ADD CONSTRAINT phone_numbers_tenant_id_id_key UNIQUE USING INDEX phone_numbers_tenant_id_id_key;
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX tags_tenant_id_id_key ON tags (tenant_id, id);
-- squawk-ignore disallowed-unique-constraint
ALTER TABLE tags ADD CONSTRAINT tags_tenant_id_id_key UNIQUE USING INDEX tags_tenant_id_id_key;
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX users_tenant_id_id_key ON users (tenant_id, id);
-- squawk-ignore disallowed-unique-constraint
ALTER TABLE users ADD CONSTRAINT users_tenant_id_id_key UNIQUE USING INDEX users_tenant_id_id_key;
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX webhook_endpoints_tenant_id_id_key ON webhook_endpoints (tenant_id, id);
-- squawk-ignore disallowed-unique-constraint
ALTER TABLE webhook_endpoints ADD CONSTRAINT webhook_endpoints_tenant_id_id_key UNIQUE USING INDEX webhook_endpoints_tenant_id_id_key;
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX whatsapp_accounts_tenant_id_id_key ON whatsapp_accounts (tenant_id, id);
-- squawk-ignore disallowed-unique-constraint
ALTER TABLE whatsapp_accounts ADD CONSTRAINT whatsapp_accounts_tenant_id_id_key UNIQUE USING INDEX whatsapp_accounts_tenant_id_id_key;

-- Children reference (tenant_id, id); the old single-column keys go.
-- squawk-ignore ban-drop-constraint
ALTER TABLE phone_numbers DROP CONSTRAINT phone_numbers_account_id_fkey;
ALTER TABLE phone_numbers ADD CONSTRAINT phone_numbers_account_id_fkey FOREIGN KEY (tenant_id, account_id) REFERENCES whatsapp_accounts (tenant_id, id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE message_templates DROP CONSTRAINT message_templates_account_id_fkey;
ALTER TABLE message_templates ADD CONSTRAINT message_templates_account_id_fkey FOREIGN KEY (tenant_id, account_id) REFERENCES whatsapp_accounts (tenant_id, id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE conversations DROP CONSTRAINT conversations_contact_id_fkey;
ALTER TABLE conversations ADD CONSTRAINT conversations_contact_id_fkey FOREIGN KEY (tenant_id, contact_id) REFERENCES contacts (tenant_id, id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE conversations DROP CONSTRAINT conversations_phone_number_id_fkey;
ALTER TABLE conversations ADD CONSTRAINT conversations_phone_number_id_fkey FOREIGN KEY (tenant_id, phone_number_id) REFERENCES phone_numbers (tenant_id, id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE messages DROP CONSTRAINT messages_contact_id_fkey;
ALTER TABLE messages ADD CONSTRAINT messages_contact_id_fkey FOREIGN KEY (tenant_id, contact_id) REFERENCES contacts (tenant_id, id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE messages DROP CONSTRAINT messages_phone_number_id_fkey;
ALTER TABLE messages ADD CONSTRAINT messages_phone_number_id_fkey FOREIGN KEY (tenant_id, phone_number_id) REFERENCES phone_numbers (tenant_id, id) ON DELETE RESTRICT NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE messages DROP CONSTRAINT messages_campaign_id_fkey;
ALTER TABLE messages ADD CONSTRAINT messages_campaign_id_fkey FOREIGN KEY (tenant_id, campaign_id) REFERENCES campaigns (tenant_id, id) ON DELETE SET NULL (campaign_id) NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE campaigns DROP CONSTRAINT campaigns_phone_number_id_fkey;
ALTER TABLE campaigns ADD CONSTRAINT campaigns_phone_number_id_fkey FOREIGN KEY (tenant_id, phone_number_id) REFERENCES phone_numbers (tenant_id, id) ON DELETE RESTRICT NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE campaign_recipients DROP CONSTRAINT campaign_recipients_campaign_id_fkey;
ALTER TABLE campaign_recipients ADD CONSTRAINT campaign_recipients_campaign_id_fkey FOREIGN KEY (tenant_id, campaign_id) REFERENCES campaigns (tenant_id, id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE campaign_recipients DROP CONSTRAINT campaign_recipients_contact_id_fkey;
ALTER TABLE campaign_recipients ADD CONSTRAINT campaign_recipients_contact_id_fkey FOREIGN KEY (tenant_id, contact_id) REFERENCES contacts (tenant_id, id) ON DELETE SET NULL (contact_id) NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE automation_runs DROP CONSTRAINT automation_runs_automation_id_fkey;
ALTER TABLE automation_runs ADD CONSTRAINT automation_runs_automation_id_fkey FOREIGN KEY (tenant_id, automation_id) REFERENCES automations (tenant_id, id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE automation_runs DROP CONSTRAINT automation_runs_contact_id_fkey;
ALTER TABLE automation_runs ADD CONSTRAINT automation_runs_contact_id_fkey FOREIGN KEY (tenant_id, contact_id) REFERENCES contacts (tenant_id, id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE contact_tags DROP CONSTRAINT contact_tags_contact_id_fkey;
ALTER TABLE contact_tags ADD CONSTRAINT contact_tags_contact_id_fkey FOREIGN KEY (tenant_id, contact_id) REFERENCES contacts (tenant_id, id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE contact_tags DROP CONSTRAINT contact_tags_tag_id_fkey;
ALTER TABLE contact_tags ADD CONSTRAINT contact_tags_tag_id_fkey FOREIGN KEY (tenant_id, tag_id) REFERENCES tags (tenant_id, id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE contact_group_members DROP CONSTRAINT contact_group_members_group_id_fkey;
ALTER TABLE contact_group_members ADD CONSTRAINT contact_group_members_group_id_fkey FOREIGN KEY (tenant_id, group_id) REFERENCES contact_groups (tenant_id, id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE contact_group_members DROP CONSTRAINT contact_group_members_contact_id_fkey;
ALTER TABLE contact_group_members ADD CONSTRAINT contact_group_members_contact_id_fkey FOREIGN KEY (tenant_id, contact_id) REFERENCES contacts (tenant_id, id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE conversation_notes DROP CONSTRAINT conversation_notes_conversation_id_fkey;
ALTER TABLE conversation_notes ADD CONSTRAINT conversation_notes_conversation_id_fkey FOREIGN KEY (tenant_id, conversation_id) REFERENCES conversations (tenant_id, id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE user_roles DROP CONSTRAINT user_roles_user_id_fkey;
ALTER TABLE user_roles ADD CONSTRAINT user_roles_user_id_fkey FOREIGN KEY (tenant_id, user_id) REFERENCES users (tenant_id, id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE user_sessions DROP CONSTRAINT user_sessions_user_id_fkey;
ALTER TABLE user_sessions ADD CONSTRAINT user_sessions_user_id_fkey FOREIGN KEY (tenant_id, user_id) REFERENCES users (tenant_id, id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE webhook_deliveries DROP CONSTRAINT webhook_deliveries_endpoint_id_fkey;
ALTER TABLE webhook_deliveries ADD CONSTRAINT webhook_deliveries_endpoint_id_fkey FOREIGN KEY (tenant_id, endpoint_id) REFERENCES webhook_endpoints (tenant_id, id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE webhook_deliveries DROP CONSTRAINT webhook_deliveries_event_id_fkey;
ALTER TABLE webhook_deliveries ADD CONSTRAINT webhook_deliveries_event_id_fkey FOREIGN KEY (tenant_id, event_id) REFERENCES outgoing_webhook_events (tenant_id, id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE media_files DROP CONSTRAINT media_files_message_id_fkey;
ALTER TABLE media_files ADD CONSTRAINT media_files_message_id_fkey FOREIGN KEY (tenant_id, message_id) REFERENCES messages (tenant_id, id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE ai_models DROP CONSTRAINT ai_models_provider_id_fkey;
ALTER TABLE ai_models ADD CONSTRAINT ai_models_provider_id_fkey FOREIGN KEY (tenant_id, provider_id) REFERENCES ai_providers (tenant_id, id) ON DELETE CASCADE NOT VALID;

-- +goose Down
-- Down migrations intentionally drop what Up created.
-- squawk-ignore ban-drop-constraint
ALTER TABLE ai_models DROP CONSTRAINT IF EXISTS ai_models_provider_id_fkey;
ALTER TABLE ai_models ADD CONSTRAINT ai_models_provider_id_fkey FOREIGN KEY (provider_id) REFERENCES ai_providers (id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE media_files DROP CONSTRAINT IF EXISTS media_files_message_id_fkey;
ALTER TABLE media_files ADD CONSTRAINT media_files_message_id_fkey FOREIGN KEY (message_id) REFERENCES messages (id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE webhook_deliveries DROP CONSTRAINT IF EXISTS webhook_deliveries_event_id_fkey;
ALTER TABLE webhook_deliveries ADD CONSTRAINT webhook_deliveries_event_id_fkey FOREIGN KEY (event_id) REFERENCES outgoing_webhook_events (id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE webhook_deliveries DROP CONSTRAINT IF EXISTS webhook_deliveries_endpoint_id_fkey;
ALTER TABLE webhook_deliveries ADD CONSTRAINT webhook_deliveries_endpoint_id_fkey FOREIGN KEY (endpoint_id) REFERENCES webhook_endpoints (id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE user_sessions DROP CONSTRAINT IF EXISTS user_sessions_user_id_fkey;
ALTER TABLE user_sessions ADD CONSTRAINT user_sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE user_roles DROP CONSTRAINT IF EXISTS user_roles_user_id_fkey;
ALTER TABLE user_roles ADD CONSTRAINT user_roles_user_id_fkey FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE conversation_notes DROP CONSTRAINT IF EXISTS conversation_notes_conversation_id_fkey;
ALTER TABLE conversation_notes ADD CONSTRAINT conversation_notes_conversation_id_fkey FOREIGN KEY (conversation_id) REFERENCES conversations (id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE contact_group_members DROP CONSTRAINT IF EXISTS contact_group_members_contact_id_fkey;
ALTER TABLE contact_group_members ADD CONSTRAINT contact_group_members_contact_id_fkey FOREIGN KEY (contact_id) REFERENCES contacts (id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE contact_group_members DROP CONSTRAINT IF EXISTS contact_group_members_group_id_fkey;
ALTER TABLE contact_group_members ADD CONSTRAINT contact_group_members_group_id_fkey FOREIGN KEY (group_id) REFERENCES contact_groups (id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE contact_tags DROP CONSTRAINT IF EXISTS contact_tags_tag_id_fkey;
ALTER TABLE contact_tags ADD CONSTRAINT contact_tags_tag_id_fkey FOREIGN KEY (tag_id) REFERENCES tags (id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE contact_tags DROP CONSTRAINT IF EXISTS contact_tags_contact_id_fkey;
ALTER TABLE contact_tags ADD CONSTRAINT contact_tags_contact_id_fkey FOREIGN KEY (contact_id) REFERENCES contacts (id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE automation_runs DROP CONSTRAINT IF EXISTS automation_runs_contact_id_fkey;
ALTER TABLE automation_runs ADD CONSTRAINT automation_runs_contact_id_fkey FOREIGN KEY (contact_id) REFERENCES contacts (id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE automation_runs DROP CONSTRAINT IF EXISTS automation_runs_automation_id_fkey;
ALTER TABLE automation_runs ADD CONSTRAINT automation_runs_automation_id_fkey FOREIGN KEY (automation_id) REFERENCES automations (id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE campaign_recipients DROP CONSTRAINT IF EXISTS campaign_recipients_contact_id_fkey;
ALTER TABLE campaign_recipients ADD CONSTRAINT campaign_recipients_contact_id_fkey FOREIGN KEY (contact_id) REFERENCES contacts (id) ON DELETE SET NULL NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE campaign_recipients DROP CONSTRAINT IF EXISTS campaign_recipients_campaign_id_fkey;
ALTER TABLE campaign_recipients ADD CONSTRAINT campaign_recipients_campaign_id_fkey FOREIGN KEY (campaign_id) REFERENCES campaigns (id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE campaigns DROP CONSTRAINT IF EXISTS campaigns_phone_number_id_fkey;
ALTER TABLE campaigns ADD CONSTRAINT campaigns_phone_number_id_fkey FOREIGN KEY (phone_number_id) REFERENCES phone_numbers (id) ON DELETE RESTRICT NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE messages DROP CONSTRAINT IF EXISTS messages_campaign_id_fkey;
ALTER TABLE messages ADD CONSTRAINT messages_campaign_id_fkey FOREIGN KEY (campaign_id) REFERENCES campaigns (id) ON DELETE SET NULL NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE messages DROP CONSTRAINT IF EXISTS messages_phone_number_id_fkey;
ALTER TABLE messages ADD CONSTRAINT messages_phone_number_id_fkey FOREIGN KEY (phone_number_id) REFERENCES phone_numbers (id) ON DELETE RESTRICT NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE messages DROP CONSTRAINT IF EXISTS messages_contact_id_fkey;
ALTER TABLE messages ADD CONSTRAINT messages_contact_id_fkey FOREIGN KEY (contact_id) REFERENCES contacts (id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE conversations DROP CONSTRAINT IF EXISTS conversations_phone_number_id_fkey;
ALTER TABLE conversations ADD CONSTRAINT conversations_phone_number_id_fkey FOREIGN KEY (phone_number_id) REFERENCES phone_numbers (id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE conversations DROP CONSTRAINT IF EXISTS conversations_contact_id_fkey;
ALTER TABLE conversations ADD CONSTRAINT conversations_contact_id_fkey FOREIGN KEY (contact_id) REFERENCES contacts (id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE message_templates DROP CONSTRAINT IF EXISTS message_templates_account_id_fkey;
ALTER TABLE message_templates ADD CONSTRAINT message_templates_account_id_fkey FOREIGN KEY (account_id) REFERENCES whatsapp_accounts (id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE phone_numbers DROP CONSTRAINT IF EXISTS phone_numbers_account_id_fkey;
ALTER TABLE phone_numbers ADD CONSTRAINT phone_numbers_account_id_fkey FOREIGN KEY (account_id) REFERENCES whatsapp_accounts (id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-constraint
ALTER TABLE ai_providers DROP CONSTRAINT IF EXISTS ai_providers_tenant_id_id_key;
-- squawk-ignore ban-drop-constraint
ALTER TABLE automations DROP CONSTRAINT IF EXISTS automations_tenant_id_id_key;
-- squawk-ignore ban-drop-constraint
ALTER TABLE campaigns DROP CONSTRAINT IF EXISTS campaigns_tenant_id_id_key;
-- squawk-ignore ban-drop-constraint
ALTER TABLE contact_groups DROP CONSTRAINT IF EXISTS contact_groups_tenant_id_id_key;
-- squawk-ignore ban-drop-constraint
ALTER TABLE contacts DROP CONSTRAINT IF EXISTS contacts_tenant_id_id_key;
-- squawk-ignore ban-drop-constraint
ALTER TABLE conversations DROP CONSTRAINT IF EXISTS conversations_tenant_id_id_key;
-- squawk-ignore ban-drop-constraint
ALTER TABLE messages DROP CONSTRAINT IF EXISTS messages_tenant_id_id_key;
-- squawk-ignore ban-drop-constraint
ALTER TABLE outgoing_webhook_events DROP CONSTRAINT IF EXISTS outgoing_webhook_events_tenant_id_id_key;
-- squawk-ignore ban-drop-constraint
ALTER TABLE phone_numbers DROP CONSTRAINT IF EXISTS phone_numbers_tenant_id_id_key;
-- squawk-ignore ban-drop-constraint
ALTER TABLE tags DROP CONSTRAINT IF EXISTS tags_tenant_id_id_key;
-- squawk-ignore ban-drop-constraint
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_tenant_id_id_key;
-- squawk-ignore ban-drop-constraint
ALTER TABLE webhook_endpoints DROP CONSTRAINT IF EXISTS webhook_endpoints_tenant_id_id_key;
-- squawk-ignore ban-drop-constraint
ALTER TABLE whatsapp_accounts DROP CONSTRAINT IF EXISTS whatsapp_accounts_tenant_id_id_key;
-- squawk-ignore ban-drop-column
ALTER TABLE outbox_events DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE whatsapp_accounts DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE whatsapp_account_events DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE webhook_events DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE webhook_endpoints DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE webhook_deliveries DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE users DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE user_sessions DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE user_roles DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE user_recovery_codes DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE user_invitations DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE tags DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE settings DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE segments DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE portfolio_reach DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE phone_numbers DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE password_reset_tokens DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE outgoing_webhook_events DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE offers DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE meta_user_preferences DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE meta_cost_daily DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE messages DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE message_templates DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE media_files DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE idempotency_keys DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE conversations DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE conversation_notes DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE conversation_handoffs DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE conversation_events DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE conversation_ai_drafts DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE conversation_activity DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE contacts DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE contact_tags DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE contact_notes DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE contact_imports DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE contact_groups DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE contact_group_members DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE contact_field_sources DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE consent_events DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE campaigns DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE campaign_reply_classifications DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE campaign_recipients DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE campaign_pauses DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE campaign_forecasts DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE campaign_exports DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE campaign_events DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE campaign_counters DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE automations DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE automation_sends DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE automation_runs DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE automation_run_steps DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE audit_logs DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE api_keys DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE ai_usage DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE ai_providers DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE ai_profiles DROP COLUMN IF EXISTS tenant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE ai_models DROP COLUMN IF EXISTS tenant_id;
