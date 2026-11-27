-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30min';

-- Phase 17 WP1 step 4 of 4: row-level security. Every tenant-owned table
-- is limited to the connection's tenant (app_tenant_id()) for reads and
-- writes, FORCE included so that even a connection as the owner obeys the
-- policy unless it is a superuser. The platform tables (tenants,
-- tenant_domains, platform_settings, the permission catalogue and the
-- price tables) have no tenant. TestEveryTableIsTenantScoped fails when a
-- new table is neither tenant-scoped nor on its platform allowlist.

ALTER TABLE ai_models ENABLE ROW LEVEL SECURITY;
ALTER TABLE ai_models FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON ai_models USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE ai_profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE ai_profiles FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON ai_profiles USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE ai_providers ENABLE ROW LEVEL SECURITY;
ALTER TABLE ai_providers FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON ai_providers USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE ai_usage ENABLE ROW LEVEL SECURITY;
ALTER TABLE ai_usage FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON ai_usage USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE api_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE api_keys FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON api_keys USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE audit_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_logs FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON audit_logs USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE automation_run_steps ENABLE ROW LEVEL SECURITY;
ALTER TABLE automation_run_steps FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON automation_run_steps USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE automation_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE automation_runs FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON automation_runs USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE automation_sends ENABLE ROW LEVEL SECURITY;
ALTER TABLE automation_sends FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON automation_sends USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE automations ENABLE ROW LEVEL SECURITY;
ALTER TABLE automations FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON automations USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE campaign_counters ENABLE ROW LEVEL SECURITY;
ALTER TABLE campaign_counters FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON campaign_counters USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE campaign_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE campaign_events FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON campaign_events USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE campaign_exports ENABLE ROW LEVEL SECURITY;
ALTER TABLE campaign_exports FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON campaign_exports USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE campaign_forecasts ENABLE ROW LEVEL SECURITY;
ALTER TABLE campaign_forecasts FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON campaign_forecasts USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE campaign_pauses ENABLE ROW LEVEL SECURITY;
ALTER TABLE campaign_pauses FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON campaign_pauses USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE campaign_recipients ENABLE ROW LEVEL SECURITY;
ALTER TABLE campaign_recipients FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON campaign_recipients USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE campaign_reply_classifications ENABLE ROW LEVEL SECURITY;
ALTER TABLE campaign_reply_classifications FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON campaign_reply_classifications USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE campaigns ENABLE ROW LEVEL SECURITY;
ALTER TABLE campaigns FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON campaigns USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE consent_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE consent_events FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON consent_events USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE contact_field_sources ENABLE ROW LEVEL SECURITY;
ALTER TABLE contact_field_sources FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON contact_field_sources USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE contact_group_members ENABLE ROW LEVEL SECURITY;
ALTER TABLE contact_group_members FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON contact_group_members USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE contact_groups ENABLE ROW LEVEL SECURITY;
ALTER TABLE contact_groups FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON contact_groups USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE contact_imports ENABLE ROW LEVEL SECURITY;
ALTER TABLE contact_imports FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON contact_imports USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE contact_notes ENABLE ROW LEVEL SECURITY;
ALTER TABLE contact_notes FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON contact_notes USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE contact_tags ENABLE ROW LEVEL SECURITY;
ALTER TABLE contact_tags FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON contact_tags USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE contacts ENABLE ROW LEVEL SECURITY;
ALTER TABLE contacts FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON contacts USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE conversation_activity ENABLE ROW LEVEL SECURITY;
ALTER TABLE conversation_activity FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON conversation_activity USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE conversation_ai_drafts ENABLE ROW LEVEL SECURITY;
ALTER TABLE conversation_ai_drafts FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON conversation_ai_drafts USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE conversation_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE conversation_events FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON conversation_events USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE conversation_handoffs ENABLE ROW LEVEL SECURITY;
ALTER TABLE conversation_handoffs FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON conversation_handoffs USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE conversation_notes ENABLE ROW LEVEL SECURITY;
ALTER TABLE conversation_notes FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON conversation_notes USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE conversations ENABLE ROW LEVEL SECURITY;
ALTER TABLE conversations FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON conversations USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE idempotency_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE idempotency_keys FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON idempotency_keys USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE media_files ENABLE ROW LEVEL SECURITY;
ALTER TABLE media_files FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON media_files USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE message_templates ENABLE ROW LEVEL SECURITY;
ALTER TABLE message_templates FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON message_templates USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE messages FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON messages USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE meta_cost_daily ENABLE ROW LEVEL SECURITY;
ALTER TABLE meta_cost_daily FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON meta_cost_daily USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE meta_user_preferences ENABLE ROW LEVEL SECURITY;
ALTER TABLE meta_user_preferences FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON meta_user_preferences USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE offers ENABLE ROW LEVEL SECURITY;
ALTER TABLE offers FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON offers USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE outgoing_webhook_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE outgoing_webhook_events FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON outgoing_webhook_events USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE password_reset_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE password_reset_tokens FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON password_reset_tokens USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE phone_numbers ENABLE ROW LEVEL SECURITY;
ALTER TABLE phone_numbers FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON phone_numbers USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE portfolio_reach ENABLE ROW LEVEL SECURITY;
ALTER TABLE portfolio_reach FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON portfolio_reach USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE segments ENABLE ROW LEVEL SECURITY;
ALTER TABLE segments FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON segments USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE settings ENABLE ROW LEVEL SECURITY;
ALTER TABLE settings FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON settings USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE tags ENABLE ROW LEVEL SECURITY;
ALTER TABLE tags FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON tags USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE user_invitations ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_invitations FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON user_invitations USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE user_recovery_codes ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_recovery_codes FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON user_recovery_codes USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE user_roles ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_roles FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON user_roles USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE user_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_sessions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON user_sessions USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE users FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON users USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE webhook_deliveries ENABLE ROW LEVEL SECURITY;
ALTER TABLE webhook_deliveries FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON webhook_deliveries USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE webhook_endpoints ENABLE ROW LEVEL SECURITY;
ALTER TABLE webhook_endpoints FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON webhook_endpoints USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE webhook_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE webhook_events FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON webhook_events USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE whatsapp_account_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE whatsapp_account_events FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON whatsapp_account_events USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE whatsapp_accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE whatsapp_accounts FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON whatsapp_accounts USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

ALTER TABLE outbox_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE outbox_events FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON outbox_events USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

-- Privileges. messora_app reads and writes tenant data (limited by the
-- policies above) and only reads the platform tables it needs for routing;
-- messora_system additionally bypasses row-level security (role attribute,
-- 00024). Tables created by later migrations get the same table privileges.
GRANT USAGE ON SCHEMA public TO messora_app, messora_system;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO messora_app, messora_system;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO messora_app, messora_system;
REVOKE INSERT, UPDATE, DELETE ON tenants, tenant_domains, platform_settings, goose_db_version FROM messora_app;
REVOKE INSERT, UPDATE, DELETE ON goose_db_version FROM messora_system;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO messora_app, messora_system;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO messora_app, messora_system;

-- +goose Down
ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE ALL ON SEQUENCES FROM messora_app, messora_system;
ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE ALL ON TABLES FROM messora_app, messora_system;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM messora_app, messora_system;
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM messora_app, messora_system;
REVOKE USAGE ON SCHEMA public FROM messora_app, messora_system;
DROP POLICY IF EXISTS tenant_isolation ON outbox_events;
ALTER TABLE outbox_events NO FORCE ROW LEVEL SECURITY;
ALTER TABLE outbox_events DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON whatsapp_accounts;
ALTER TABLE whatsapp_accounts NO FORCE ROW LEVEL SECURITY;
ALTER TABLE whatsapp_accounts DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON whatsapp_account_events;
ALTER TABLE whatsapp_account_events NO FORCE ROW LEVEL SECURITY;
ALTER TABLE whatsapp_account_events DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON webhook_events;
ALTER TABLE webhook_events NO FORCE ROW LEVEL SECURITY;
ALTER TABLE webhook_events DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON webhook_endpoints;
ALTER TABLE webhook_endpoints NO FORCE ROW LEVEL SECURITY;
ALTER TABLE webhook_endpoints DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON webhook_deliveries;
ALTER TABLE webhook_deliveries NO FORCE ROW LEVEL SECURITY;
ALTER TABLE webhook_deliveries DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON users;
ALTER TABLE users NO FORCE ROW LEVEL SECURITY;
ALTER TABLE users DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON user_sessions;
ALTER TABLE user_sessions NO FORCE ROW LEVEL SECURITY;
ALTER TABLE user_sessions DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON user_roles;
ALTER TABLE user_roles NO FORCE ROW LEVEL SECURITY;
ALTER TABLE user_roles DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON user_recovery_codes;
ALTER TABLE user_recovery_codes NO FORCE ROW LEVEL SECURITY;
ALTER TABLE user_recovery_codes DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON user_invitations;
ALTER TABLE user_invitations NO FORCE ROW LEVEL SECURITY;
ALTER TABLE user_invitations DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON tags;
ALTER TABLE tags NO FORCE ROW LEVEL SECURITY;
ALTER TABLE tags DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON settings;
ALTER TABLE settings NO FORCE ROW LEVEL SECURITY;
ALTER TABLE settings DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON segments;
ALTER TABLE segments NO FORCE ROW LEVEL SECURITY;
ALTER TABLE segments DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON portfolio_reach;
ALTER TABLE portfolio_reach NO FORCE ROW LEVEL SECURITY;
ALTER TABLE portfolio_reach DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON phone_numbers;
ALTER TABLE phone_numbers NO FORCE ROW LEVEL SECURITY;
ALTER TABLE phone_numbers DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON password_reset_tokens;
ALTER TABLE password_reset_tokens NO FORCE ROW LEVEL SECURITY;
ALTER TABLE password_reset_tokens DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON outgoing_webhook_events;
ALTER TABLE outgoing_webhook_events NO FORCE ROW LEVEL SECURITY;
ALTER TABLE outgoing_webhook_events DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON offers;
ALTER TABLE offers NO FORCE ROW LEVEL SECURITY;
ALTER TABLE offers DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON meta_user_preferences;
ALTER TABLE meta_user_preferences NO FORCE ROW LEVEL SECURITY;
ALTER TABLE meta_user_preferences DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON meta_cost_daily;
ALTER TABLE meta_cost_daily NO FORCE ROW LEVEL SECURITY;
ALTER TABLE meta_cost_daily DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON messages;
ALTER TABLE messages NO FORCE ROW LEVEL SECURITY;
ALTER TABLE messages DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON message_templates;
ALTER TABLE message_templates NO FORCE ROW LEVEL SECURITY;
ALTER TABLE message_templates DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON media_files;
ALTER TABLE media_files NO FORCE ROW LEVEL SECURITY;
ALTER TABLE media_files DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON idempotency_keys;
ALTER TABLE idempotency_keys NO FORCE ROW LEVEL SECURITY;
ALTER TABLE idempotency_keys DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON conversations;
ALTER TABLE conversations NO FORCE ROW LEVEL SECURITY;
ALTER TABLE conversations DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON conversation_notes;
ALTER TABLE conversation_notes NO FORCE ROW LEVEL SECURITY;
ALTER TABLE conversation_notes DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON conversation_handoffs;
ALTER TABLE conversation_handoffs NO FORCE ROW LEVEL SECURITY;
ALTER TABLE conversation_handoffs DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON conversation_events;
ALTER TABLE conversation_events NO FORCE ROW LEVEL SECURITY;
ALTER TABLE conversation_events DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON conversation_ai_drafts;
ALTER TABLE conversation_ai_drafts NO FORCE ROW LEVEL SECURITY;
ALTER TABLE conversation_ai_drafts DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON conversation_activity;
ALTER TABLE conversation_activity NO FORCE ROW LEVEL SECURITY;
ALTER TABLE conversation_activity DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON contacts;
ALTER TABLE contacts NO FORCE ROW LEVEL SECURITY;
ALTER TABLE contacts DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON contact_tags;
ALTER TABLE contact_tags NO FORCE ROW LEVEL SECURITY;
ALTER TABLE contact_tags DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON contact_notes;
ALTER TABLE contact_notes NO FORCE ROW LEVEL SECURITY;
ALTER TABLE contact_notes DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON contact_imports;
ALTER TABLE contact_imports NO FORCE ROW LEVEL SECURITY;
ALTER TABLE contact_imports DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON contact_groups;
ALTER TABLE contact_groups NO FORCE ROW LEVEL SECURITY;
ALTER TABLE contact_groups DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON contact_group_members;
ALTER TABLE contact_group_members NO FORCE ROW LEVEL SECURITY;
ALTER TABLE contact_group_members DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON contact_field_sources;
ALTER TABLE contact_field_sources NO FORCE ROW LEVEL SECURITY;
ALTER TABLE contact_field_sources DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON consent_events;
ALTER TABLE consent_events NO FORCE ROW LEVEL SECURITY;
ALTER TABLE consent_events DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON campaigns;
ALTER TABLE campaigns NO FORCE ROW LEVEL SECURITY;
ALTER TABLE campaigns DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON campaign_reply_classifications;
ALTER TABLE campaign_reply_classifications NO FORCE ROW LEVEL SECURITY;
ALTER TABLE campaign_reply_classifications DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON campaign_recipients;
ALTER TABLE campaign_recipients NO FORCE ROW LEVEL SECURITY;
ALTER TABLE campaign_recipients DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON campaign_pauses;
ALTER TABLE campaign_pauses NO FORCE ROW LEVEL SECURITY;
ALTER TABLE campaign_pauses DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON campaign_forecasts;
ALTER TABLE campaign_forecasts NO FORCE ROW LEVEL SECURITY;
ALTER TABLE campaign_forecasts DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON campaign_exports;
ALTER TABLE campaign_exports NO FORCE ROW LEVEL SECURITY;
ALTER TABLE campaign_exports DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON campaign_events;
ALTER TABLE campaign_events NO FORCE ROW LEVEL SECURITY;
ALTER TABLE campaign_events DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON campaign_counters;
ALTER TABLE campaign_counters NO FORCE ROW LEVEL SECURITY;
ALTER TABLE campaign_counters DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON automations;
ALTER TABLE automations NO FORCE ROW LEVEL SECURITY;
ALTER TABLE automations DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON automation_sends;
ALTER TABLE automation_sends NO FORCE ROW LEVEL SECURITY;
ALTER TABLE automation_sends DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON automation_runs;
ALTER TABLE automation_runs NO FORCE ROW LEVEL SECURITY;
ALTER TABLE automation_runs DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON automation_run_steps;
ALTER TABLE automation_run_steps NO FORCE ROW LEVEL SECURITY;
ALTER TABLE automation_run_steps DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON audit_logs;
ALTER TABLE audit_logs NO FORCE ROW LEVEL SECURITY;
ALTER TABLE audit_logs DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON api_keys;
ALTER TABLE api_keys NO FORCE ROW LEVEL SECURITY;
ALTER TABLE api_keys DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON ai_usage;
ALTER TABLE ai_usage NO FORCE ROW LEVEL SECURITY;
ALTER TABLE ai_usage DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON ai_providers;
ALTER TABLE ai_providers NO FORCE ROW LEVEL SECURITY;
ALTER TABLE ai_providers DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON ai_profiles;
ALTER TABLE ai_profiles NO FORCE ROW LEVEL SECURITY;
ALTER TABLE ai_profiles DISABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON ai_models;
ALTER TABLE ai_models NO FORCE ROW LEVEL SECURITY;
ALTER TABLE ai_models DISABLE ROW LEVEL SECURITY;
