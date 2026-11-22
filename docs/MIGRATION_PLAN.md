# Geçiş Planı — ZeroMsg (Laravel/Inertia) → Messora (Go/Next.js)

> Phase 0 · 2026-11-22 · Bu doküman *kodun ve işlevlerin* taşınmasını tanımlar. Çalışan bir ZeroMsg kurulumundan *veri* taşıma §5'tedir.

## 1. Strateji

- **Yeniden yazım, satır satır port değil.** ZeroMsg'nin iş mantığı küçüktür (~2.900 satır PHP). Ancak güvenilirlik ve güvenlik açıkları (F1–F20) yapısal sorunlardır: senkron webhook işleme, durum makinesi olmaması, düz metin secret'lar. Bu yüzden backend Go'da yeniden tasarlanır. ZeroMsg'nin **davranışı** (ne yaptığı) referans alınır, **uygulaması** (nasıl yaptığı) alınmaz.
- **Frontend: bileşen ve görünüm korunur, veri katmanı değişir.** UI kodu (JSX, Tailwind sınıfları, shadcn bileşenleri) Next.js'e taşınır ve görsel tasarım değişmez. Inertia'ya bağlı noktalar (`usePage`, `useForm`, `router`, `Link`, Wayfinder) adaptörlerle değiştirilir.
- **Her phase sonunda çalışan bir uygulama:** Phase 1'den itibaren `docker compose up` stack'i ayağa kaldırır. Henüz yazılmamış modüller UI'da "henüz mevcut değil" olarak görünmez, menüde hiç yer almaz. Yarım ekran yayınlanmaz.

## 2. Backend modül eşlemesi

| ZeroMsg kaynağı | Messora modülü | Taşınan davranış | Değişen / eklenen |
|---|---|---|---|
| Fortify, `CreateNewUser`, `ResetUserPassword`, `LoginResponse`, `SecurityController`, `ProfileController` | `auth`, `users` | login, logout, şifre sıfırlama, 2FA, profil, parola değişikliği, rate limit | Açık kayıt kaldırılır, yerine ilk kurulum + davet gelir. Argon2id. Oturum yönetimi. |
| `EnsureUserIsClient`, `role` kolonu | `roles` | — | RBAC sıfırdan yazılır |
| — | `audit` | — | Yeni |
| `WhatsAppCloudApi` | `whatsapp` | Tüm mevcut çağrılar | Medya, tüm mesaj tipleri, hata sınıflandırması, timeout, `biz_opaque_callback_data`, sürüm yapılandırması |
| `WhatsappAccountController` | `channels` | Manuel bağlama, WABA sync, numara detayları, güncelleme, silme | Token şifreleme, kullanıcı PIN'i ile register (F19). Embedded Signup ertelenir. |
| `WebhookHandler`, `WhatsAppWebhookController` | `whatsapp` (alım) + `messaging` + `conversations` + `automation` | İmza doğrulama, inbound mesaj, durum güncellemesi, kalite bildirimi | Önce kaydet sonra işle, idempotent yan etkiler, monoton durum, verify token, template durum event'i |
| `MessageTemplate` (model metotları), `MessageTemplateController`, `SyncTemplateStatus` | `templates` | Oluşturma, silme, sync, **parametre eşleme mantığı** (`getBodyParameters`, `getTemplateComponents`, `formatBodyText`) birebir davranışla | Periyodik sync, HEADER parametreleri, tablo testleri (ZeroMsg girdi/çıktı örnekleriyle) |
| `CampaignController`, `ProcessCampaign`, `SendCampaignMessage`, `DispatchScheduledCampaigns` | `campaigns` + `messaging` | Draft, planlama (timezone), grup veya CSV ile alıcı, template/direct, start/pause, istatistik | Pencereli dispatcher, resume (F1), cancel, retry, unknown durumu, opt-out, dedupe, segment kitlesi |
| `ContactController`, `ContactGroupController` | `contacts` | CRUD, arama, grup filtresi, gruplar, CSV import (`name, phone, email, notes, var1..5` başlıkları otomatik eşlenir), CSV export (UTF-8 BOM) | E.164, unique telefon, tag, not, custom field, XLSX, worker'da import ve rapor |
| — | `segments` | — | Yeni |
| `InboxController` | `conversations` | Liste, geçmiş, text/template yanıt, 24h pencere hesaplaması | Pagination, atama, kapatma, medya, okunmamış sayısı, realtime |
| `AutomationFlowController`, `WebhookHandler::processAutomationFlows/executeGraphNode/...` | `automation` | Keyword tetikleyici, graf yürütme semantiği (koşul dallanması: OR'da eşleşen koşulun indeksi → kenar indeksi), tüm çalışan aksiyonlar, import/export v1 | Worker'da çalıştırma, gecikme task'i, yeni tetikleyiciler, döngü koruması, execution log |
| `DeveloperController`, `AuthenticateApiKey`, `MessageApiController` | `api_keys`, `messaging` (API), `webhooks` | API key yaşam döngüsü, `send-text`, `send-template`, giden webhook (`message.received`, HMAC) | Scope, rate limit, Idempotency-Key, yeni event'ler, teslimat logu, SSRF koruması |
| `DashboardController` | `campaigns` + `conversations` (stats endpoint'leri) | Tüm dashboard metrikleri | Ön hesaplanmış günlük sayaçlar |
| `SystemNotification`, `HandleInertiaRequests::share` | `notifications`, `realtime` | Bildirim listesi, okundu işaretleme, toast | Centrifugo ile anlık iletim |

### 2.1 Geriye dönük uyumluluk sözleşmeleri

Kullanıcının ZeroMsg'den taşıyacağı yapıtlar Messora'da çalışmaya devam etmelidir:

1. **Otomasyon export dosyaları** (`type: zeromsg.automation_flow, version: 1`): Messora bu formatı import eder ve `automations.graph`'a dönüştürür. Desteklenmeyen aksiyonlar (`google_sheets`) import sırasında raporlanır ve devre dışı node olarak eklenir. Messora kendi export'unu `type: messora.automation, version: 2` olarak yazar.
2. **Contact CSV başlıkları:** `name, phone, email, notes, var1..var5` otomatik eşlenir. `name`, `first_name`/`last_name` alanlarına ilk boşluktan bölünür.
3. **Kampanya alıcı CSV'si:** İlk kolon telefon, sonraki kolonlar pozisyonel template değişkenleridir. Bu format korunur.
4. **Template değişkenleri:** `{{1}}` ve named `{{isim}}` eşlemesi aynı davranır. Grup kampanyasında ZeroMsg'nin varsayılan değişken sırası (`name, var1..var5`) yeni kampanyada varsayılan eşleme olarak önerilir.
5. **Developer API:** `POST /api/v1/messages/send-text` ve `/send-template` istek gövdesi (`phone_number_id, to, body` / `template_name, language, variables`) **aynen kabul edilir**. Yanıt yeni zarf formatındadır. Deprecation notu OpenAPI'de belirtilir, yeni birleşik endpoint `POST /api/v1/messages` olur.
6. **Giden webhook payload'ı** (`event, timestamp, data.message_id, data.sender, data.message`) `message.received` için korunur. `tenant_id` alanı kaldırılır.
7. **API key öneki:** Eski `zm_live_` key'leri veri taşımayla gelirse kabul edilir (hash aynı, SHA-256). Yeni key'ler `msk_live_` önekiyle üretilir.

## 3. Frontend taşıma planı

### 3.1 Dosya eşlemesi

| ZeroMsg (`resources/js`) | Messora (`frontend/src`) | İşlem |
|---|---|---|
| `components/ui/*` (33) | `components/ui/*` | Aynen kopya (+ `'use client'` gereken yerlerde) |
| `components/*` (30): app-sidebar, nav, breadcrumbs, heading, input-error, data-table, status-badge … | `components/*` | Kopya. `Link`/`usePage` kullananlar adaptöre bağlanır. |
| `layouts/*` | `app/(panel)/layout.tsx`, `app/(auth)/layout.tsx`, `app/(panel)/settings/layout.tsx` | Next layout'larına dönüştürülür |
| `hooks/*` (8) | `hooks/*` | Kopya. `use-appearance` cookie mantığı korunur. |
| `pages/auth/*` (7) | `app/(auth)/{login,forgot-password,reset-password,two-factor,setup,invite}/page.tsx` | `register` ve `verify-email` kaldırılır, yerine `setup` ve `invite` gelir |
| `pages/client/dashboard.tsx` | `app/(panel)/page.tsx` + client bileşen | Veri server component'ten gelir |
| `pages/client/*/index.tsx`, `show.tsx`, `create.tsx` | `app/(panel)/<modül>/…/page.tsx` | Aynı desen |
| `pages/client/automations/flow-builder.tsx` (2.915 satır) | `components/automation/flow-builder.tsx` (client) | En az değişiklikle; kayıt işlemi Server Action'a bağlanır |
| `pages/settings/*` | `app/(panel)/settings/*` | + sessions, users, roles, api-keys, channels, webhooks, audit sayfaları |
| `types/*` | `types/*` + `lib/api/schema.d.ts` (OpenAPI'den üretilir) | Elle yazılan tipler üretilen tiplerle değiştirilir |
| — | `lib/api/server.ts` (server-only fetch), `lib/realtime.ts` (Centrifuge client singleton), `lib/inertia-shim.ts` | Yeni |

### 3.2 Inertia shim

Değişiklik yüzeyini küçültmek için geçici adaptörler yazılır:
- `usePage().props.auth` yerine `useSession()` (layout'tan context ile sağlanır)
- `useForm` yerine aynı imzaya yakın bir `useActionForm(serverAction)` (`data`, `setData`, `errors`, `processing`, `submit`)
- `router.reload` yerine `router.refresh()`
- Inertia `Link` yerine `next/link`

Böylece sayfa bileşenleri büyük ölçüde mekanik olarak taşınır. Shim'ler Phase 9 sonunda gerçek implementasyona dönüşür, geçici kod bırakılmaz.

### 3.3 Polling'in kaldırılması

- Inbox'taki 5 sn polling yerine `conversation:<id>` ve `inbox:all` aboneliği gelir. Kopmada API'den yeniden çekilir.
- Kampanya ekranındaki manuel Refresh yerine `campaign:<id>` aboneliği gelir. Buton yedek olarak kalır.

## 4. Doğrulama: "işlev kaybı yok" kontrolü

`FEATURE_MATRIX.md`'de `Keep` veya `Fix` olarak işaretlenen her satır için ilgili phase'de en az bir otomatik test yazılır. Phase 10'da matris tekrar gözden geçirilir ve her satırın yanına test adı veya E2E senaryosu yazılır. Karşılığı olmayan satır kalırsa release yapılmaz.

## 5. ZeroMsg kurulumundan veri taşıma (opsiyonel araç)

`messora import zeromsg --source "mysql://…|postgres://…|sqlite:///…" --tenant-id N` (Phase 10, ROADMAP):
- Tek bir tenant seçilir (Messora tek işletmedir).
- Taşınanlar: users (parolalar bcrypt hash olarak taşınır ve ilk girişte Argon2id'e yükseltilir), contacts (`var1..5` → custom fields), groups, templates, whatsapp hesapları (token'lar okunup **şifrelenerek** yazılır), chats ve messages, campaigns (salt okunur geçmiş olarak), automations, api_keys, outgoing_webhooks.
- Taşınmayanlar: sessions, jobs, cache, passkeys (WebAuthn credential'ları RP ID'ye bağlıdır).
- Dry-run modu ve satır sayısı karşılaştırma raporu sunulur.

## 6. Lisans yükümlülükleri

- ZeroMsg MIT lisanslıdır (Copyright (c) 2026 megoxv). MIT, kaynak koddan türetilmiş "substantial portions" içeren dağıtımlarda telif ve izin metninin korunmasını şart koşar.
- Messora'ya taşınacak ZeroMsg kaynaklı kodlar şunlardır: frontend UI bileşenleri ve sayfa JSX'i (en büyük kısım), flow-builder ve template parametre eşleme mantığı. Bunlar türetilmiş eser sayılır.
- **Uygulama:**
  - Messora'nın kendi lisansı MIT olacak (talimat §21; ZeroMsg ile uyumlu).
  - Kök dizine `NOTICE` (veya `THIRD_PARTY_NOTICES.md`) eklenecek. İçinde ZeroMsg'nin tam MIT metni ve telif satırı ile hangi dizinlerin ZeroMsg'den türetildiği yer alacak.
  - ZeroMsg'den kopyalanan dosyalarda üstte kısa bir köken notu olacak (`// Derived from ZeroMsg (MIT) — see NOTICE`).
  - README'de "ZeroMsg'den türetilmiştir" teşekkür bölümü olacak.
- **Üçüncü parti bağımlılık lisansları:** shadcn/ui (MIT), Radix (MIT), xyflow (MIT), lucide (ISC), Tailwind (MIT), Next.js (MIT), chi (MIT), pgx (MIT), sqlc (MIT, araç), goose (MIT), asynq (MIT), Centrifugo (Apache-2.0, ayrı container), go-redis (BSD-2), phonenumbers (MIT), excelize (BSD-3) — hepsi MIT dağıtımla uyumlu.
  - Redis 8: RSALv2/SSPLv1/AGPLv3 lisanslı ama ayrı bir container olarak, değiştirilmeden kullanılıyor. Messora'nın lisansını etkilemez; README'de belirtilecek, Valkey (BSD) alternatif olarak sunulacak.
  - Lisans taraması (`go-licenses`, `license-checker`) CI'a eklenecek.
- ZeroMsg'nin `art/` klasöründeki banner ve screenshot'lar ile logo **taşınmaz**. Messora kendi markasını kullanır.
