# Veritabanı Tasarımı (PostgreSQL 18)

> DDL Phase 1–8 boyunca goose migration'ları olarak modül modül yazılacak. Bu doküman hedef şemayı ve gerekçeleri tanımlar.

## 1. Genel kurallar

| Konu | Karar |
|---|---|
| ID | `uuid PRIMARY KEY DEFAULT uuidv7()` (PG 18 yerleşik). Zaman sıralı olduğu için B-tree'de insert yerelliği iyi ve keyset pagination için `ORDER BY id` yeterli. Yalnızca `roles`, `permissions`, `settings` gibi küçük referans tablolarında doğal anahtar (`key text`). |
| Zaman | Her yerde `timestamptz`, UTC yazılır. Kullanıcı timezone'u yalnızca gösterim ve planlama girdisi için (`campaigns.timezone`). |
| Enum | PG `ENUM` yerine `text` + `CHECK (x IN (...))` kullanılır. Değer eklemek tek satırlık migration olur, `ALTER TYPE` kısıtları yaşanmaz. |
| E-posta | `citext` (büyük/küçük harf duyarsız unique). |
| Telefon | `phone_e164 text CHECK (phone_e164 ~ '^\+[1-9][0-9]{6,14}$')`. Normalizasyon uygulamada libphonenumber ile yapılır. |
| Para | `numeric(14,6)` + `currency char(3)` (maliyet takibi geldiğinde). |
| Secret | `bytea` olarak saklanır, AES-256-GCM, format `key_version(1B) ‖ nonce(12B) ‖ ciphertext`. Tablo/kolon adına `_enc` son eki verilir. |
| Token | Yalnızca `sha256` hash'i saklanır (`bytea`, unique). |
| JSONB | Yalnızca gerçekten yapısı değişken olan alanlarda: Meta payload'ları, template component'leri, otomasyon grafı, segment kural ağacı, custom field değerleri. Her biri aşağıda gerekçelendirildi. |
| FK | Tüm ilişkiler FK ile tanımlanır. `ON DELETE` davranışı tablo bazında açıkça belirtilir. FK kolonlarının hepsi indekslenir. |
| Soft delete | Varsayılan olarak **yok**. Kullanıcılar `disabled` olur (audit bütünlüğü için). Contact'lar gizlilik gereği hard delete edilir (KVKK/GDPR silme hakkı), audit log'da iz kalır. Kampanyalar başlatıldıktan sonra silinemez, yalnızca `archived` olur. |
| Migration | goose, `migrations/NNNNN_<ad>.sql`, `-- +goose Up/Down`. Büyük tablolarda `CREATE INDEX CONCURRENTLY` (`-- +goose NO TRANSACTION`). Kolon silme iki adımda yapılır (önce kullanmayı bırak, sonraki sürümde düş). |
| Extension | `citext`, `pg_trgm` (contact arama). |

### 1.1 Kiracılar (Phase 17 WP1)

- **Kayıt tabloları:** `tenants` (`slug` alt alan adı etiketidir: 3–40 karakter, küçük harf, rakam ve iç tire, ayrılmış adlar `tenant_reserved_slugs()`; `status` `active | suspended | deleted`), `tenant_domains` (her alan adı tek kiracıya aittir; `kind` `subdomain | custom`; `verified_at` boşsa alan adı yanıt vermez; kiracı başına tek `is_primary`), `platform_settings` (WP2 platform paneli). Bunlarda RLS yoktur; `messora_app` yalnızca okur.
- **`tenant_id`:** platform tabloları (`tenants`, `tenant_domains`, `platform_settings`, `permissions`, `roles`, `role_permissions`, `meta_rates`, `meta_rate_markets`, `ai_model_prices`, `goose_db_version`) dışındaki 57 tablonun hepsinde `tenant_id uuid NOT NULL REFERENCES tenants ON DELETE RESTRICT DEFAULT app_tenant_id()` bulunur. `outbox_events.tenant_id` boş olabilir (platform olayları); uygulama rolü yine de yalnızca kendi kiracısının olayını yazıp okuyabilir.
- **Yazma:** sütun varsayılanı bağlantının kiracısıdır (`app_tenant_id()`, havuz kancasının ayarladığı `app.tenant_id`); sorgular değeri ayrıca geçirmez. Kiracısız bağlantıda varsayılan `NULL` olur ve `NOT NULL` ekleme yapılamaz; RLS'in `WITH CHECK` ifadesi başka bir kiracının değerini reddeder. Trigger'lar yazdıkları satıra `NEW.tenant_id`'yi açıkça kopyalar (`ops_hint`, `phone_number_quality_event`, `message_template_change_event`, `campaign_counters_init`, `campaign_counters_recompute`).
- **Okuma:** RLS (`FORCE`) her sorguya `tenant_id = app_tenant_id()` ekler. ID ile okuma da kiracı içinde kalır.
- **Bileşik anahtarlar:** ebeveynlerde `UNIQUE (tenant_id, id)`; çocuklar `FOREIGN KEY (tenant_id, x_id) REFERENCES parent (tenant_id, id)` ile bağlanır. `ON DELETE SET NULL (x_id)` yalnızca ilişki sütununu boşaltır.
- **Kiracı bazlı tekiller:** `settings (tenant_id, key)` birincil anahtar; `contacts (tenant_id, phone_e164)`; etiket, grup, segment adları `(tenant_id, lower(name))`; kiracı başına tek varsayılan numara; `ai_providers (tenant_id, name)`; `messages (tenant_id, wamid)`; `webhook_events (tenant_id, payload_sha256)`; `meta_user_preferences`; açık davet `(tenant_id, email)`; `idempotency_keys (tenant_id, principal, key)`. **Global kalanlar** (yönlendirme): `whatsapp_accounts.waba_id`, `phone_numbers.phone_number_id`, bütün token ve anahtar hash'leri, `users.email` (WP2'ye kadar).
- **Danışma kilidi:** sahip değişikliği kilidi kiracıyı içerir (`messora.owner_changes:<tenant>`).
- **Roller:** sahip (`messora`) migrasyonları çalıştırır; `messora_app` (`NOBYPASSRLS`) servislerin rolüdür; `messora_system` (`BYPASSRLS`) yalnızca outbox aktarıcısı ve temizliği içindir. Sonraki migrasyonların tabloları `ALTER DEFAULT PRIVILEGES` ile iki role de DML hakkı alır; yeni bir platform tablosu ekleyen migrasyon `messora_app`'in yazma hakkını açıkça geri almalıdır.
- **Mevcut verinin taşınması (00024–00028):** `tenant_id` önce boş eklenir, mevcut satırlar tek transaction'da 1 numaralı kiracıya (`00000000-0000-0000-0000-000000000001`) taşınır, `CHECK (tenant_id IS NOT NULL) NOT VALID` eklenir; sonraki migrasyonda doğrulanır, `SET NOT NULL` taramasız yapılır ve kontrol düşürülür. 1 numaralı kiracının `slug`'ı organizasyon adından türetilir (`Acme Şirketi` → `acme-sirketi`, kullanılamıyorsa `default`). Alan adlarını `messora migrate` kaydeder: `<slug>.TENANT_BASE_DOMAIN` ve `PUBLIC_APP_URL` host'u. Down migrasyonları tersine çalışır (iki kiracı aynı anahtarı paylaşıyorsa 00027 Down bilerek başarısız olur).
- **Meta bağlantısı (WP5, 00030):** `meta_connections` kiracı başına tek satırdır (RLS'li): webhook anahtarı ve doğrulama belirteci `Sealer` ile şifreli, sağlık sayaçları (son webhook, imza hataları, bağlı olmayan WABA olayları). `meta_webhook_keys` platform yönlendirme tablosudur (RLS yok, `tenant_domains` gibi): anahtarın SHA-256'sı → kiracı, eskiyen anahtar `expires_at`'e kadar geçerli; kiracı başına tek güncel anahtar. `messora_app` bu tabloyu yalnızca okur; yazma yalnızca `meta_webhook_key_rotate(sha256, grace)` (`SECURITY DEFINER`, satırı çağıran bağlantının `app.tenant_id`'sine yazar) ile olur. `platform_settings.tech_provider` (`enabled: false`) WP6'nın anahtarıdır.
- **Tech Provider (WP6, 00032):** `whatsapp_accounts.connection_method` (`own_app` / `tech_provider`). `meta_routes` platform yönlendirme tablosudur (RLS yok): WABA → kiracı; `messora_app` yalnızca okur, yazma `meta_route_claim(waba)` / `meta_route_release(waba)` (`SECURITY DEFINER`, çağıranın `app.tenant_id`'si) ile. `meta_onboardings` (RLS'li) Embedded Signup'ın sürdürülebilir durum makinesidir (adım, durum, hata kodu; business token ve PIN şifreli), WABA başına tek açık kayıt. `platform_settings.tech_provider` sırları `Sealer` ile şifreli base64 alanlarda tutar.
- **Planlar ve yetkiler (WP8, 00034):** platform tabloları `plans` (katalog: `limits`/`features` JSON, gösterim fiyatları, `active`/`public`), `platform_ai_providers` (sağlayıcı türü başına Messora anahtarı, şifreli) ve `platform_ai_models`; `messora_app` okur, `messora_platform` yazar. `tenants.plan_key` (FK, NULL = varsayılan plan), `plan_overrides` (firmaya özel limitler), `plan_changed_at`. Kiracı tabloları (RLS): `ai_credit_lots` (milikredi lotları: `plan` ayda bir `(tenant_id, period_start)` tekil, `grant`, `purchase`; tüketimde sabit sırayla `FOR UPDATE`) ve `plan_change_requests` (firma başına tek `open`; platform rolüne `platform_access`). `ai_usage.key_source`/`platform_provider_id`/`credits`, `ai_profiles.platform_provider_id`, devir nedeni `ai_credits`. Ayrıntı: docs/ENTITLEMENTS.md, docs/AI.md.
- **Platform tohumları:** AI fiyat tablosunu `messora migrate` sahip rolüyle tohumlar; işaret `platform_settings.ai_prices_seeded`'dadır (00029).
- **CI kontrolü:** `TestEveryTableIsTenantScoped` yeni bir tablonun kiracıya bağlanmadan eklenmesini engeller.

### 1.2 Global hesaplar ve platform tabloları (Phase 17 WP2, migrasyon 00031)

- **Hesap global, rol üyelikte:** `users` artık kiracıya bağlı değildir (RLS yok); `email` (`citext`) bütün platformda tekildir. `email_verified_at` (kayıtta boş; davet ve sıfırlama bağlantısı doğrular), `is_platform_admin`, `password_reset_required` eklendi. `user_recovery_codes` de kişiye aittir. `messora_app` hesapları okur ve günceller (giriş, profil, parola, TOTP), davet kabulünde oluşturur, **silemez**.
- **`tenant_memberships (tenant_id, user_id, role_id, status, invited_by, created_at, updated_at)`**, birincil anahtar `(tenant_id, user_id)`, RLS'li. Firmadaki rol ve durum (`active | disabled`) buradadır; `user_roles` kaldırıldı. Taşıma: her kullanıcı kendi kiracısına en yetkili rolüyle ve eski durumuyla üye oldu, hesabı aktif kaldı.
- **Oturumlar:** `user_sessions` kiracıya bağlı kalır; `user_id → users(id)` (bileşik anahtar yok: destek oturumunun üyeliği yoktur). `support_grant_id → support_access_grants` ve `read_only` destek oturumunu işaretler. Üye oturumu her istekte etkin üyelik ister.
- **`tenants`:** `subscription_status` (`trial | active | past_due | canceled`, varsayılan `active`), `trial_ends_at`, `suspended_reason` (≤ 500). Yaşam döngüsü (`status`) ayrıdır. `tenant_reserved_slugs()` artık kayıt formunun listesidir (12 ad); yeni slug kuralı uygulamada 3–30 karakter, eski satırlar yeniden denetlenmez.
- **`tenant_domains`:** `verification_token` (`messora-verify=<hex>`, `_messora.<alan>` TXT kaydı), `last_checked_at`, `last_check_error`.
- **Platform tabloları (RLS yok, `messora_app` erişemez):** `platform_sessions` (platform panelinin oturumları; `token_hash` tekil, 12 saat), `email_verification_tokens` (`user_id`, giriş yapılacak firma `tenant_id`, `token_hash`, `expires_at`, `used_at`), `platform_audit_logs` (`actor_id`, `actor_email`, `action`, `entity_type`, `entity_id`, ilgili firma `tenant_id`, istek bilgileri, `changes`). Bu üçünde ve `tenant_domains`'te `tenant_id` "hangi firma hakkında" demektir, satırı kiracıya bağlamaz.
- **`support_access_grants (tenant_id, admin_id, reason, read_only, created_at, expires_at, ended_at)`:** kiracıya bağlı ve RLS'li (firma kendi kayıtlarını görür); `messora_app` yalnızca okur, platform rolü yazar.
- **`messora_platform` rolü:** `NOLOGIN NOBYPASSRLS`, `messora_app`'e `WITH INHERIT FALSE, SET TRUE`. Yetkileri: kayıt tabloları, katalog ve platform tablolarında DML; `users` SELECT/INSERT/UPDATE; `tenant_memberships`, `user_sessions`, `support_access_grants`'ta `platform_access` politikası (bütün kiracılar); `settings`, `user_invitations`, `audit_logs`, `password_reset_tokens`'ta yalnızca transaction'ın kiracısı (`set_config('app.tenant_id', …, true)`). `meta_rates`, `meta_rate_markets`, `ai_model_prices` artık yalnızca platform rolüyle yazılır.
- **Down:** 00031 Down hesapları ilk üyeliklerinin kiracısına geri taşır; birden çok firmada üyeliği olan kişi varsa bilerek başarısız olur.

## 2. Kimlik, yetki, sistem

```sql
-- Phase 17 WP2: users is global; roles live in tenant_memberships (§1.2).
users (
  id uuid PK, email citext UNIQUE NOT NULL, name text NOT NULL,
  password_hash text NOT NULL,                 -- argon2id PHC string
  status text CHECK (status IN ('active','disabled')) DEFAULT 'active',
  totp_secret_enc bytea, totp_confirmed_at timestamptz,
  locale text DEFAULT 'en', timezone text DEFAULT 'UTC',
  last_login_at timestamptz, created_at, updated_at
)
roles (id uuid PK, key text UNIQUE, name text, is_system bool, created_at)      -- owner, admin, manager, agent
permissions (key text PK, module text, description text)                          -- 'contacts.read' …
role_permissions (role_id FK→roles CASCADE, permission_key FK→permissions CASCADE, PK(role_id, permission_key))
user_roles (user_id FK→users CASCADE, role_id FK→roles RESTRICT, PK(user_id, role_id))
user_sessions (
  id uuid PK, user_id FK→users CASCADE, token_hash bytea UNIQUE NOT NULL,
  mfa_passed_at timestamptz, ip inet, user_agent text,
  created_at, last_seen_at, expires_at, revoked_at
)  INDEX (user_id) ; INDEX (expires_at) WHERE revoked_at IS NULL
user_recovery_codes (id, user_id FK CASCADE, code_hash bytea, used_at)
user_invitations (id, email citext, role_id FK, token_hash bytea UNIQUE, invited_by FK→users, expires_at, accepted_at, revoked_at)
password_reset_tokens (id, user_id FK CASCADE, token_hash bytea UNIQUE, expires_at, used_at)
settings (key text PK, value jsonb NOT NULL, updated_by FK→users SET NULL, updated_at)
api_keys (
  id uuid PK, name text, prefix text NOT NULL,          -- 'msk_live_ab12' gösterim için
  key_hash bytea UNIQUE NOT NULL, scopes text[] NOT NULL,
  created_by FK→users SET NULL, last_used_at, expires_at, revoked_at, created_at
)
idempotency_keys (
  principal text, key text, request_hash bytea, status text CHECK (in_progress|done),
  response_status int, response_body jsonb, created_at, PRIMARY KEY (principal, key)
)  INDEX (created_at)                                    -- 24 saat sonra temizlenir
audit_logs (
  id uuid PK, occurred_at timestamptz NOT NULL,
  actor_type text CHECK (user|api_key|system), actor_id uuid,
  action text NOT NULL,                                  -- 'campaign.start', 'user.role_change' …
  entity_type text, entity_id uuid, request_id text, ip inet, user_agent text,
  changes jsonb                                          -- {"field": [old, new]}, secret'lar asla yazılmaz
)  INDEX (entity_type, entity_id, occurred_at DESC) ; INDEX (actor_id, occurred_at DESC) ; BRIN (occurred_at)
notifications (id, user_id FK CASCADE, type, title, body, data jsonb, toast_shown_at, read_at, created_at)
  INDEX (user_id, created_at DESC) WHERE read_at IS NULL
outbox_events (id bigint GENERATED ALWAYS AS IDENTITY PK, topic text, payload jsonb, available_at, published_at, attempts int, created_at)
  INDEX (id) WHERE published_at IS NULL
```

- **Rol/izin:** `roles`, `permissions`, `role_permissions`, `user_roles` ve `users`. Sistem rolleri (owner, admin, manager, agent) seed migration'ı ile yüklenir ve silinemez. Özel rol oluşturma ilk sürümde UI'da yoktur ama model destekler.
- `outbox_events.id` `bigint` identity'dir. Sıralı tüketim için ucuz ve hiçbir zaman dışarı açılmaz.

## 3. WhatsApp

```sql
whatsapp_accounts (
  id uuid PK, name text, meta_app_id text, app_secret_enc bytea,
  waba_id text UNIQUE NOT NULL, business_id text,
  access_token_enc bytea NOT NULL, token_last4 text, token_expires_at timestamptz,
  status text CHECK (active|needs_attention|disabled), status_reason text,
  created_at, updated_at
)
whatsapp_phone_numbers (
  id uuid PK, account_id FK→whatsapp_accounts RESTRICT,
  phone_number_id text UNIQUE NOT NULL, display_phone_e164 text, verified_name text,
  quality_rating text, messaging_limit_tier text, throughput_mps int DEFAULT 80,
  status text CHECK (active|pending|banned|disconnected), is_default bool DEFAULT false,
  registered_at, last_synced_at, created_at, updated_at
)  UNIQUE (is_default) WHERE is_default                 -- en fazla bir varsayılan numara
whatsapp_templates (
  id uuid PK, account_id FK RESTRICT, meta_template_id text,
  name text, language text, category text CHECK (MARKETING|UTILITY|AUTHENTICATION),
  status text,                                           -- approved|pending|rejected|paused|disabled
  components jsonb NOT NULL, parameter_format text, rejection_reason text,
  quality_score text, last_synced_at, created_at, updated_at,
  UNIQUE (account_id, name, language)
)
whatsapp_webhook_events (
  id uuid PK, received_at timestamptz NOT NULL, body_sha256 bytea UNIQUE NOT NULL,
  payload jsonb NOT NULL, processed_at timestamptz, attempts int DEFAULT 0, last_error text
)  INDEX (received_at) WHERE processed_at IS NULL ; BRIN (received_at)   -- saklama 30 gün (ayarlanabilir)
whatsapp_media (
  id uuid PK, meta_media_id text UNIQUE, direction text CHECK (inbound|outbound),
  mime_type text, size_bytes bigint, sha256 bytea, filename text,
  storage_key text, status text CHECK (pending|stored|failed|expired), created_at
)
```

- `whatsapp_accounts` ve `whatsapp_phone_numbers` ayrıdır. Token WABA seviyesinde, throughput ve kalite numara seviyesinde tutulur.
- `components jsonb`: Meta'nın template şemasıdır ve sürümle değişir. Alanları tek tek normalize etmenin faydası yok. Gerekçeli JSONB kullanımı.
- `whatsapp_webhook_events`: Meta zaten tekrar gönderdiği için body hash'i dedupe için yeterlidir. Ham kayıt hata ayıklama ve yeniden işleme (replay) için saklanır.

## 4. Contacts ve segmentasyon

```sql
contacts (
  id uuid PK, phone_e164 text UNIQUE NOT NULL, country_code char(2),
  first_name text, last_name text, email citext,
  custom_fields jsonb NOT NULL DEFAULT '{}',
  consent_status text CHECK (unknown|opted_in|opted_out) DEFAULT 'unknown',
  consent_updated_at timestamptz,
  last_inbound_at timestamptz, last_outbound_at timestamptz,
  source text, created_at, updated_at
)  INDEX gin ((coalesce(first_name,'')||' '||coalesce(last_name,'')||' '||phone_e164||' '||coalesce(email::text,'')) gin_trgm_ops)
   INDEX (country_code) ; INDEX (consent_status) ; INDEX (last_inbound_at) ; INDEX (created_at)
   INDEX gin (custom_fields jsonb_path_ops)
custom_field_definitions (key text PK CHECK (key ~ '^[a-z][a-z0-9_]{0,39}$'), label text, type text CHECK (text|number|date|boolean|select), options jsonb, position int)
tags (id uuid PK, name citext UNIQUE, color text, created_at)
contact_tags (contact_id FK CASCADE, tag_id FK CASCADE, created_at, created_by, PK(contact_id, tag_id))  INDEX (tag_id, contact_id)
contact_groups (id uuid PK, name citext UNIQUE, description text, created_at, updated_at)
contact_group_members (group_id FK CASCADE, contact_id FK CASCADE, added_at, PK(group_id, contact_id))  INDEX (contact_id)
contact_notes (id uuid PK, contact_id FK CASCADE, author_id FK→users SET NULL, body text, created_at, updated_at)  INDEX (contact_id, created_at DESC)
contact_consent_events (
  id uuid PK, contact_id FK CASCADE, status text CHECK (opted_in|opted_out),
  source text CHECK (manual|import|keyword|api|campaign_reply|webhook),
  evidence text, actor_type text, actor_id uuid, occurred_at
)  INDEX (contact_id, occurred_at DESC)
contact_imports (
  id uuid PK, filename text, format text CHECK (csv|xlsx), storage_key text,
  column_mapping jsonb, options jsonb,                  -- group_ids, tag_ids, update_existing, default_country, consent
  status text CHECK (uploaded|queued|running|completed|failed|cancelled),
  total_rows int, processed_rows int, created_count int, updated_count int, skipped_count int, failed_count int,
  error_report_key text, created_by FK, cancel_requested_at, started_at, finished_at, created_at
)
contact_import_errors (import_id FK CASCADE, row_number int, error_code text, raw jsonb, PK(import_id, row_number))   -- en fazla 10.000 satır; tam rapor dosyada
segments (
  id uuid PK, name text UNIQUE, description text,
  rule jsonb NOT NULL, rule_version int NOT NULL DEFAULT 1,
  estimated_count int, estimated_at timestamptz, created_by FK, created_at, updated_at
)
```

- **Telefon unique:** Tek işletme olduğu için `phone_e164` global unique'tir (duplicate kayıtların kökten çözümü). Import işlemi `ON CONFLICT (phone_e164) DO UPDATE/NOTHING` ile çalışır (kullanıcı seçimine göre).
- **`custom_fields jsonb`:** Alan tanımları `custom_field_definitions`'ta durur ve tipler uygulamada doğrulanır.var5` anahtarlı tanımlar olarak taşınır, böylece mevcut template değişken eşlemeleri çalışmaya devam eder. EAV tablosu yerine JSONB seçildi, çünkü segment sorgularında join patlaması yaratmıyor ve GIN index ile filtrelenebiliyor.
- **Opt-in kayıtları:** `contacts.consent_status` hızlı filtre içindir. Kanıt ve geçmiş `contact_consent_events`'te tutulur (kim, ne zaman, hangi kaynaktan).
- **Segmentler:**
  - `segment_rules` tablosu yerine kural ağacı `segments.rule` içinde **tipli JSONB** olarak saklanır. AND/OR ağacını tablolara bölmek her düzenlemede çok satırlı senkronizasyon gerektirir ve okuma tarafında kazanç sağlamaz. Kural uygulamada şemaya göre doğrulanır ve `rule_version` ile sürümlenir.
  - `segment_memberships` tablosu **yoktur**. Dinamik segmentler kullanım anında hesaplanır. Kampanyada sonuç `campaign_recipients` olarak donar ki bu zaten anlık görüntüdür. Materyalize üyelik tablosu güncelliğini kaybeder ve tutarsızlık kaynağı olur. Büyük kitlelerde önizleme sayısı `estimated_count` ile cache'lenir.
  - Kural örneği: `{"op":"and","rules":[{"field":"country_code","cmp":"eq","value":"TR"},{"field":"tag","cmp":"has","value":"<tag_id>"},{"field":"last_inbound_at","cmp":"older_than_days","value":30}]}`
  - Derleyici yalnızca whitelist'teki alan/operatör çiftlerini kabul eder (`country_code`, `tag`, `group`, `consent_status`, `created_at`, `last_inbound_at`, `last_outbound_at`, `custom.<key>` + tip uyumlu operatörler). Tüm değerler `$n` parametresi olarak geçer. Maksimum derinlik 5, maksimum 50 kural.

## 5. Kampanyalar

```sql
campaigns (
  id uuid PK, name text NOT NULL,
  status text CHECK (draft|scheduled|preparing|running|paused|paused_limit|completed|cancelled|failed|archived),
  phone_number_id FK→whatsapp_phone_numbers RESTRICT,
  message_kind text CHECK (template|text),
  template_id FK→whatsapp_templates RESTRICT, template_params jsonb,   -- {"body":[{"source":"contact.first_name"},{"source":"custom.var1"},{"static":"…"}],"header":…,"buttons":…}
  text_body text,
  audience_type text CHECK (segment|group|upload|contacts), audience_ref uuid, audience_upload_key text,
  scheduled_at timestamptz, timezone text,
  materialize_cursor text, last_error text,
  created_by FK, started_at, paused_at, completed_at, cancelled_at, created_at, updated_at,
  CHECK ((message_kind='template') = (template_id IS NOT NULL))
)  INDEX (status, scheduled_at) WHERE status='scheduled' ; INDEX (created_at DESC)
campaign_stats (
  campaign_id uuid PK FK CASCADE, total int, pending int, queued int, accepted int, sent int,
  delivered int, read int, failed int, unknown int, skipped int, cancelled int, updated_at
)
campaign_recipients (
  id uuid PK, campaign_id FK CASCADE, contact_id FK→contacts SET NULL,
  phone_e164 text NOT NULL, variables jsonb,            -- çözümlenmiş template parametreleri (donmuş)
  state text CHECK (pending|queued|done|skipped|cancelled),
  skip_reason text,                                      -- opted_out|invalid_phone|no_session|duplicate
  message_id uuid UNIQUE FK→messages SET NULL, created_at, updated_at,
  UNIQUE (campaign_id, phone_e164)
)  INDEX (campaign_id, state, id)                       -- dispatcher claim sorgusu
campaign_events (id uuid PK, campaign_id FK CASCADE, type text, actor_type, actor_id, data jsonb, occurred_at)  INDEX (campaign_id, occurred_at)
```

- **`campaign_messages` tablosu yok:** Kampanya mesajı da bir mesajdır, `messages` tablosunda `campaign_id` ile tutulur. İki ayrı mesaj tablosu durum güncellemelerini ve webhook uzlaştırmasını ikiye böler.
- **Alıcı durumu ile mesaj durumu ayrıdır:** `campaign_recipients.state` yalnızca kampanya iş akışının yerini (pending → queued → done) gösterir. Teslim durumu tek kaynaktan, `messages.state`'ten okunur. Recipient listesi `JOIN messages` ile gösterilir.
- **Ölçek (100k alıcı):** Materialize 1000'erlik `INSERT … SELECT` batch'leriyle yapılır, 100k alıcı ~100 sorgu demektir. Dispatcher claim sorgusu `(campaign_id, state, id)` index'ini kullanır. `campaign_stats` satır kilidi saniyede onlarca güncellemeyi rahatça kaldırır.
- `template_params jsonb`: Template'e göre şekil değiştiren eşleme tanımı. Gerekçeli JSONB kullanımı.

## 6. Konuşmalar ve mesajlar

```sql
conversations (
  id uuid PK, phone_number_id FK RESTRICT, contact_id FK→contacts SET NULL,
  customer_phone_e164 text NOT NULL, customer_profile_name text,
  status text CHECK (open|closed) DEFAULT 'open',
  assigned_user_id FK→users SET NULL,
  last_message_at timestamptz, last_inbound_at timestamptz, customer_window_expires_at timestamptz,
  unread_count int DEFAULT 0, automation_muted_until timestamptz,
  closed_at, created_at, updated_at,
  UNIQUE (phone_number_id, customer_phone_e164)
)  INDEX (status, last_message_at DESC) ; INDEX (assigned_user_id, status, last_message_at DESC)
conversation_assignments (id uuid PK, conversation_id FK CASCADE, user_id FK SET NULL, assigned_by FK SET NULL, assigned_at, unassigned_at)  INDEX (conversation_id, assigned_at DESC)
messages (
  id uuid PK, conversation_id FK RESTRICT, phone_number_id FK RESTRICT,
  direction text CHECK (inbound|outbound),
  kind text CHECK (text|template|image|video|audio|document|sticker|location|interactive|reaction|button|contacts|system|unsupported),
  wamid text UNIQUE,                                     -- outbound için Meta kabul edene kadar NULL
  state text CHECK (received|queued|sending|retry_wait|held|accepted|sent|delivered|read|failed|unknown|cancelled),
  status_rank smallint NOT NULL DEFAULT 0,
  body text, payload jsonb,                              -- template parametreleri, interactive yapı, location, Meta inbound ham mesaj parçası
  media_id FK→whatsapp_media SET NULL,
  source text CHECK (inbox|api|automation|campaign|webhook),
  campaign_id FK→campaigns SET NULL, automation_execution_id uuid, sent_by_user_id FK→users SET NULL, api_key_id FK→api_keys SET NULL,
  attempts smallint DEFAULT 0, lease_until timestamptz, next_attempt_at timestamptz,
  error_code text, error_title text, error_detail text,
  created_at, accepted_at, sent_at, delivered_at, read_at, failed_at, updated_at
)  INDEX (conversation_id, id DESC)                    -- mesaj geçmişi (keyset)
   INDEX (next_attempt_at) WHERE state='retry_wait'
   INDEX (lease_until) WHERE state='sending'             -- reaper
   INDEX (created_at) WHERE state='unknown'
   INDEX (campaign_id, state) WHERE campaign_id IS NOT NULL
   BRIN (created_at)
message_status_events (
  id uuid PK, message_id FK CASCADE, status text, occurred_at timestamptz, received_at timestamptz,
  error jsonb, pricing jsonb, UNIQUE (message_id, status)
)
message_attempts (
  id uuid PK, message_id FK CASCADE, attempt smallint, started_at, finished_at,
  outcome text CHECK (accepted|retryable|permanent|ambiguous), http_status int, error_code text, duration_ms int,
  UNIQUE (message_id, attempt)
)
```

- Durum geçmişi `message_status_events` tablosundadır. `(message_id, status)` unique olduğu için tekrarlanan webhook'lar idempotent işlenir.
- Konuşma, `(numara, müşteri)` başına **tek satırdır**. Kapatılan konuşma yeni mesajla yeniden açılır, geçmiş bölünmez.
- `customer_window_expires_at = last_inbound_at + 24h` alanı inbound mesaj işlenirken güncellenir. Inbox, API ve kampanyadaki tüm 24h kontrolleri bu tek alana bakar.
- **Büyüme:** 100k'lık kampanyalar düzenli çalışırsa `messages` yılda on milyonlarca satıra ulaşır. v1'de partition yok. Bunun sebebi `wamid` global unique kısıtının partition anahtarı içermesi gerekmesi, bu da dedupe garantisini bozar. UUIDv7 + BRIN + kısmi index'ler bu ölçekte yeterli. Arşivleme ve partition stratejisi (`messages` → aylık partition + ayrı `message_wamids` dedupe tablosu) ROADMAP'te "ölçek" maddesi olarak duruyor.

## 7. Otomasyon

```sql
automations (
  id uuid PK, name text, is_active bool DEFAULT false,
  trigger_type text CHECK (message_received|keyword|contact_created|tag_added|conversation_opened),
  trigger_config jsonb,                                  -- {"keyword":"fiyat","match":"contains"} | {"tag_id":"…"}
  graph jsonb NOT NULL,                                  -- {nodes, edges}: görsel akış editörünün dokümanı
  version int NOT NULL DEFAULT 1, cooldown_seconds int DEFAULT 0, max_runs_per_contact_per_day int,
  created_by FK, created_at, updated_at
)  INDEX (trigger_type) WHERE is_active
automation_executions (
  id uuid PK, automation_id FK CASCADE, automation_version int,
  trigger_key text NOT NULL,                             -- 'msg:<message_id>' | 'tag:<contact>:<tag>:<ts>' …
  contact_id FK SET NULL, conversation_id FK SET NULL,
  status text CHECK (running|waiting|completed|failed|skipped),
  skip_reason text, current_node text, depth int, resume_at timestamptz,
  started_at, finished_at, error text,
  UNIQUE (automation_id, trigger_key)                    -- aynı tetikleyici iki kez çalışamaz
)  INDEX (contact_id, started_at DESC) ; INDEX (resume_at) WHERE status='waiting'
automation_execution_steps (id uuid PK, execution_id FK CASCADE, node_id text, action_type text, status text, output jsonb, error text, started_at, finished_at)
```

- `automation_triggers` ve `automation_actions` tabloları yok. Akış editörü grafı bir bütün olarak düzenler, normalize tablolar her kayıtta çok satırlı diff gerektirir. Tetikleyici sorgulanabilir kolonlarda (`trigger_type`, index'li) tutulur. Graf bir JSONB doküman olarak `version` ile sürümlenir ve çalışan execution kendi `automation_version`'ını bilir.
- **Döngü koruması:** `UNIQUE (automation_id, trigger_key)` tekrarı engeller. Otomasyonun kendi gönderdiği mesajlar tetikleyici olamaz (yalnızca inbound tetikler). `depth` sınırı ve node başına ziyaret sayacı uygulanır. `cooldown_seconds` ve `max_runs_per_contact_per_day` ile kişi bazında frekans sınırı konur.

## 8. Giden webhook'lar

```sql
webhook_endpoints (id uuid PK, url text, description text, event_types text[], active bool,
  secret_enc bytea, secret_last4 text, disabled_reason text, created_by FK, created_at, updated_at)
  GIN INDEX (event_types) WHERE active
outgoing_webhook_events (id uuid PK, type text, payload jsonb, created_at)   INDEX (created_at)
webhook_deliveries (
  id uuid PK, event_id FK CASCADE, endpoint_id FK CASCADE,
  status text CHECK (pending|delivering|retrying|succeeded|failed), attempts int, next_attempt_at,
  locked_until, last_attempt_at, last_status_code int, last_duration_ms int, last_error text,
  last_response_body text, delivered_at, created_at, updated_at,
  UNIQUE (event_id, endpoint_id)
)  INDEX (endpoint_id, id DESC)
```

- Event, değişikliği yapan transaction'da yazılır (yalnızca aktif bir abone varsa) ve outbox'a `webhook:fanout` task'i bırakır. Fan-out her abone endpoint için bir teslimat satırı açar; `UNIQUE (event_id, endpoint_id)` tekrar eden relay'de çift teslimatı engeller.
- Retry planı Redis'te değil, `next_attempt_at` ve gecikmeli bir outbox satırında durur. `locked_until`, çalışırken ölen bir worker'ın teslimatını lease süresi sonunda başka worker'ın almasını sağlar.
- `webhook_deliveries` için ayrı `payload` yok: gövde event'ten okunur, böylece yeniden gönderim aynı event'i (aynı `id`) taşır.

## 9. Saklama ve temizlik

| Veri | Varsayılan saklama | Ayar |
|---|---|---|
| `whatsapp_webhook_events` (işlenmiş) | 30 gün | `RETENTION_WEBHOOK_EVENTS_DAYS` |
| `outbox_events` (yayınlanmış) | 7 gün | sabit |
| `idempotency_keys` | 24 saat | sabit |
| `outgoing_webhook_events` + `webhook_deliveries` | 30 gün | sabit (`webhook:cleanup`, 6 saatte bir) |
| `audit_logs` | süresiz | `RETENTION_AUDIT_DAYS` (0 = süresiz) |
| `user_sessions` (süresi dolmuş) | 7 gün sonra silinir | sabit |
| `messages`, `conversations` | süresiz | ileride arşivleme |
