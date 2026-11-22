# ZeroMsg Kaynak Kod Analizi

> Phase 0 çıktısı · 2026-11-22
> Kaynak: `https://github.com/megoxv/zeromsg` @ `ac876d4b537cffefc06da942cddf529e3dd2afa2` (yerel kopya: `reference/zeromsg/`, git-ignored)
> Lisans: **MIT** (Copyright (c) 2026 megoxv) — türetilmiş kodda telif bildirimi korunacak (bkz. `MIGRATION_PLAN.md` §6).

Bu doküman ZeroMsg'in **gerçekte ne yaptığını** belgeler. README'deki iddialar değil, kaynak kod, ZeroMsg'in kendi test paketi ve Phase 0'da yazılan doğrulama probe'ları esas alınmıştır. Özellik bazında durum tablosu: [`FEATURE_MATRIX.md`](FEATURE_MATRIX.md).

---

## 1. Doğrulama yöntemi

| Adım | Sonuç |
|---|---|
| Tüm backend kodu satır satır okundu (`app/`, `routes/`, `database/`, `config/whatsapp.php`) | ~2.900 satır PHP |
| Frontend sayfa ve bileşen yapısı incelendi | 117 dosya, ~12.000 satır sayfa kodu |
| ZeroMsg test paketi çalıştırıldı (PHP 8.3.32, SQLite in-memory) | **73/73 geçti** (290 assertion) — ön koşul olarak frontend build gerekti |
| `npm ci` | **Başarısız**: `package-lock.json`, `package.json` ile senkron değil. `npm install` ile çalıştı. |
| Phase 0 doğrulama probe'ları (10 test, şüpheli davranışları kanıtlamak için) | 10/10 bulgu doğrulandı — kaynak: [`phase0/zeromsg-verification-probes.php`](phase0/zeromsg-verification-probes.php) |

Probe'lar **mevcut (hatalı) davranışı** assert eder; geçen test = bulgu doğrulandı. Referans kopyadan kaldırıldılar.

---

## 2. Genel mimari

```text
Tarayıcı ──Inertia XHR──> Laravel 13 (PHP 8.3, Fortify)
                              │  Inertia::render() ile sayfa prop'ları
                              │  Eloquent ORM ── SQLite/MySQL/PostgreSQL
                              │  Queue (database driver varsayılan) ── Jobs
                              │  Scheduler: campaigns:dispatch-scheduled (her dakika)
Meta ──webhook──> /webhook/whatsapp/{tenant_token}  (senkron işlenir)
Geliştirici ──Bearer API key──> /api/v1/messages/*
```

- **Backend:** Laravel 13, Fortify (login, register, reset, e-posta doğrulama, TOTP 2FA, passkey). Klasik MVC; iş mantığının büyük kısmı controller'larda ve tek bir `WebhookHandler` (731 satır) içinde.
- **Frontend:** React 19 + TypeScript + Vite 8 + **Inertia.js v3** + Tailwind v4 + shadcn/Radix UI + `@xyflow/react` (otomasyon akış editörü) + `sonner` (toast).
  - **Kritik:** Frontend bir API istemcisi değildir. 53 dosya `@inertiajs/react` import eder; sayfa verisi controller'dan prop olarak gelir, formlar `useForm`/`router.post` ile Laravel route'larına gider. Ayrı bir JSON API katmanı yoktur (yalnızca 3 endpoint JSON döner: inbox mesajları, inbox gönderim, grup üyeleri, kampanya CSV parse).
  - Wayfinder (Laravel route → TS fonksiyon üretimi) kullanılır.
- **Multi-tenant:** `tenants` tablosu; her kayıt olan kullanıcıya otomatik bir tenant (workspace) açılır. Tenant izolasyonu controller'larda elle `tenant_id` karşılaştırması ile yapılır (global scope yok).
- **Gerçek zamanlı:** **Yok.** Inbox mesajları 5 sn'de bir polling (`setInterval`), kampanya ekranı manuel "Refresh" (`router.reload`). Bildirimler/toast'lar her Inertia isteğinde shared prop olarak gelir.
- **Kuyruk:** Laravel queue (`.env.example`: `QUEUE_CONNECTION=database`). Redis zorunlu değil.

---

## 3. Veritabanı (ZeroMsg)

| Tablo | Amaç | Not |
|---|---|---|
| `tenants` | Workspace | `webhook_token` (64 hex) webhook URL'inde kullanılır |
| `users` | Kullanıcı | `role` yalnızca `'client'`; `tenant_id`; 2FA alanları |
| `passkeys` | WebAuthn | Fortify passkeys |
| `password_reset_tokens`, `sessions`, `cache*`, `jobs`, `job_batches`, `failed_jobs` | Laravel altyapısı | |
| `whatsapp_accounts` | Telefon numarası + WABA + **access_token + app_secret** | **Şifrelenmemiş** (cast yok) — F6 |
| `message_templates` | Meta şablonları | `components` JSON; `(whatsapp_account_id,name,language)` unique **değil** |
| `campaigns` | Kampanya | `message_type`: `template` \| `direct`; sayaçlar; `total_cost` (hep 0) |
| `campaign_recipients` | Alıcı | `phone_number` serbest metin; contact FK yok; `(campaign_id, phone)` unique **değil** |
| `whatsapp_chats` | Konuşma | `unique(whatsapp_account_id, customer_phone)` |
| `whatsapp_messages` | Mesaj | `meta_message_id` unique; `body` text; medya yok; otomasyon mesajları sahte `auto_…` ID'si alır |
| `contacts` | Kişi | `var1..var5` sabit özel alanlar; `(tenant_id, phone)` index ama **unique değil** |
| `contact_groups`, `contact_group_memberships` | Statik gruplar | |
| `api_keys` | Geliştirici API | `key` = SHA-256 hash ✓ |
| `outgoing_webhooks` | Giden webhook | `secret_token` düz metin, UI'da gösterilir |
| `automation_flows` | Otomasyon | `actions` JSON + `visual_graph` JSON (xyflow) |
| `system_notifications` | Uygulama içi bildirim | `is_toast_shown` |

**Eksik olanlar:** roller/izinler, audit log, opt-in/opt-out, etiketler, notlar tablosu, segment, medya, webhook event log, mesaj durum geçmişi, konuşma ataması, otomasyon çalıştırma kaydı.

---

## 4. Endpoint envanteri (66 route)

Tam liste `php artisan route:list --except-vendor` çıktısından alınmıştır. Fortify route'ları (login, logout, register, forgot/reset password, email verify, 2FA, passkeys, confirm-password) ek olarak vardır.

| Grup | Route'lar | Yanıt tipi |
|---|---|---|
| Webhook | `GET/POST webhook/whatsapp/{tenant_token?}` | text / JSON |
| Developer API | `POST api/v1/messages/send-text`, `send-template` | JSON (API key) |
| Dashboard | `GET dashboard` | Inertia |
| WhatsApp hesapları | index, create(redirect), `request-code`, `{id}/verify`, `sync`, `embedded-signup`, `PUT/DELETE {id}` | Inertia/redirect |
| Contacts | index, store, update, destroy, `import` (CSV), `export` (CSV stream) | Inertia/redirect |
| Contact groups | index (**sayfa yok** — F8), store, update, destroy, `{g}/members` (JSON), `{g}/contacts` add, `{g}/contacts/{c}` remove | karışık |
| Inbox | index, `chats/{c}/messages` (JSON), `chats/{c}/send` (JSON) | karışık |
| Developer panel | index, keys store/destroy, webhooks store/toggle/destroy | Inertia/redirect |
| Automations | index, create, store, edit, update, destroy, toggle, import (JSON dosya), export | Inertia/redirect/JSON |
| Templates | index, create, store (Meta'ya gönderir), show, destroy (Meta'dan siler), sync | Inertia/redirect |
| Campaigns | index, create, store, show, start, pause, `upload-recipients` (JSON) | Inertia/redirect/JSON |
| Notifications | `{id}/read`, `read-all` | redirect |
| Settings | profile edit/update/destroy, security edit, password update, appearance | Inertia |

---

## 5. Modül bazında bulgular

### 5.1 Authentication & authorization
- Fortify: login (5/dk rate limit), register, şifre sıfırlama, e-posta doğrulama, TOTP 2FA, passkey — **çalışıyor** (test kapsamında).
- **Açık kayıt:** `/register` herkese açık; her kayıt yeni bir tenant oluşturur. Tek işletme senaryosunda bu bir güvenlik açığıdır (internete açık kurulumda herkes hesap açar).
- **RBAC yok.** `EnsureUserIsClient` middleware'i rolü olmayan herkesi `client` yapar ve tenant'ı yoksa açar.
- **Takım/çoklu kullanıcı yok:** bir tenant'a ikinci kullanıcı ekleme akışı yok. API key doğrulaması `tenant->users()->first()` ile "sahte" bir kullanıcı atar.
- Production'da güçlü parola politikası (`min 12, mixed, symbols, uncompromised`) ✓.

### 5.2 WhatsApp Cloud API istemcisi (`WhatsAppCloudApi`, 203 satır)
- Desteklenen çağrılar: template mesaj, text mesaj, `request_code`, `verify_code`, `register`, telefon detayları, WABA telefon listesi, template create/list/delete, business profile.
- **Yok:** medya (upload/download), image/video/document/audio/location/interactive mesajlar, `biz_opaque_callback_data`, hata kodu sınıflandırması, timeout ayarı (Laravel varsayılanı 30 sn), retry.
- Graph API sürümü varsayılan **v21.0** — resmî tabloya göre **21 Ocak 2027'de sona eriyor**. Ayrıca kodda sabit yazılmış `v21.0` (`requestCode`) ve `v25.0` (OAuth code exchange) URL'leri var. Güncel en yeni sürüm **v26.0** (29 Temmuz 2026).
- **Otomasyon içinden çağrı bozuk (F2):** `new WhatsAppCloudApi([...])` + `sendTextMessage($to, $text)` — metot 3 argüman ister → `ArgumentCountError`. Bu bir `Error` olduğu için `catch (\Exception)` yakalamaz → webhook isteği **HTTP 500** döner → Meta webhook'u tekrar gönderir. Token da hiç set edilmez.

### 5.3 Webhook (`WhatsAppWebhookController` + `WebhookHandler`)
- İmza doğrulama: ham body üzerinde `HMAC-SHA256`, `hash_equals` ✓. Secret seçimi: tenant'ın `app_secret`'i olan **ilk** hesabı, yoksa global `WHATSAPP_APP_SECRET`.
- **Verify challenge `hub.verify_token`'ı kontrol etmiyor (F4).** Geçerli tenant token'ı URL'de olan herkes abonelik doğrulamasını geçer.
- İşleme **senkron**, HTTP isteğinin içinde: mesaj kaydı, otomasyonlar (içinde `sleep()` olan `delay` aksiyonu dahil), giden webhook dispatch. Uzun sürerse Meta timeout → tekrar gönderim.
- Ham event saklanmıyor; işleme başarısız olursa veri kaybolur (Meta'nın retry'ına güvenilir).
- **Idempotency kısmi (F3):** mesaj `meta_message_id` unique ile tekrar oluşmaz, ancak otomasyonlar ve giden webhook'lar **her tekrarda yeniden çalışır** (çift otomatik yanıt / çift HTTP çağrısı).
- **Durum sırası korunmuyor (F5):** `read` sonra gelen `delivered` ile geri `delivered`'a düşer. Meta webhook sırası garanti etmez.
- Her durum event'inde kampanya sayaçları **6 adet COUNT sorgusu** ile baştan hesaplanır → 100k alıcılı kampanyada her webhook O(n).
- `failed` durumunda Meta'nın `errors[]` detayı kaydedilmez.
- Desteklenen inbound tipler: `text`, `button`. Diğerleri `"[Unsupported Message: image]"` metni olarak kaydedilir. `phone_number_quality_update` işlenir (bildirim üretir). `message_template_status_update` **işlenmez**.
- Bildirim metinleri **Arapça olarak sabit yazılmış** (i18n yok).

### 5.4 Kampanyalar (`CampaignController`, `ProcessCampaign`, `SendCampaignMessage`)
- Oluşturma: grup seçimi veya CSV ile alıcı listesi; alıcılar **HTTP isteği içinde** tek tek `INSERT` edilir (100k alıcı = 100k sorgu, request timeout).
- Template ve **direct (serbest metin)** kampanya. Direct kampanya 24 saat penceresi kontrolü yapmadan herkese serbest metin gönderir (F9) — Meta pencere dışında reddeder (131047), politika riski.
- Planlama: `scheduled_at` + timezone, scheduler her dakika atomik `UPDATE … WHERE status='scheduled'` ile claim eder ✓ (çift dispatch'e karşı doğru).
- `ProcessCampaign`: tüm `pending` alıcıları **belleğe yükler**, her alıcı için ayrı job dispatch eder, batch başına sabit gecikme verir. `messages_per_second` config'i **kullanılmaz**; numara başına rate limit yok.
- **Resume bozuk (F1):** `paused` kampanyada "Start" → `ProcessCampaign` statü `paused` olduğu için hemen döner. Kampanya sonsuza kadar duraklatılmış kalır.
- **İptal yok.**
- **Retry fiilen yok (F10):** `SendCampaignMessage` tüm exception'ları yakalayıp alıcıyı `failed` yapar; `$tries = 3` hiç tetiklenmez. Timeout (mesaj aslında gitmiş olabilir) = kalıcı `failed`.
- **Duplicate gönderim riski (kod analizi):** "status pending mi?" kontrolü ile API çağrısı arasında kilit yok; worker API çağrısından sonra, DB güncellemesinden önce ölürse job tekrar çalışır ve mesaj ikinci kez gider. Belirsiz durum ("gitti mi bilmiyoruz") modellenmemiş.
- Sayaçlar hem job'da `increment` hem webhook'ta COUNT ile yazılır → yarış durumu, tutarsız sayaç.
- Maliyet (`total_cost`) hiç hesaplanmaz.
- Alıcı listesinde **dedupe yok**, **opt-out kontrolü yok**.
- `upload-recipients` `xlsx/xls` kabul eder ama `fgetcsv` ile parse eder → Excel **bozuk**.

### 5.5 Contacts & groups
- CRUD, arama (`LIKE %q%`), grup filtresi, pagination ✓.
- Telefon normalizasyonu: yalnızca rakam + `+` (E.164 doğrulaması yok, ülke kodu tahmini yok).
- `(tenant_id, phone)` unique değil → aynı numaradan çok kayıt mümkün (import `updateOrCreate` kullandığı için import'ta tekrarlamaz, manuel eklemede tekrarlar).
- Import: yalnızca CSV, sabit başlık adları (`name, phone, email, notes, var1..5`), **HTTP isteği içinde senkron**, satır başına sorgu, hata raporu yok, progress yok, iptal yok.
- Export: CSV stream, UTF-8 BOM ✓; ama tüm contact'ları `get()` ile belleğe alır.
- Özel alanlar sabit 5 kolon (`var1..var5`).
- Sayfa her yüklemede **tüm contact'ları** (`allContacts`) prop olarak gönderir → büyük listede ölçeklenmez.
- **IDOR (F7):** `group_ids.*` yalnızca `exists:contact_groups,id` ile doğrulanır → başka tenant'ın grubuna bağlanabilir. Aynı desen otomasyon `group_id`'sinde de var. (Tek-tenant Messora'da tenant sınırı kalkacağı için etkisi değişir, ama yetki kontrolsüz ID kabulü deseni tekrarlanmamalı.)
- `store/update` isteğe bağlı `email`/`notes` anahtarı gönderilmezse **500** (`Undefined array key`). UI her alanı gönderdiği için arayüzde görünmez, API kullanımında görünür.
- Etiketler, notlar (çoklu), segmentler, opt-in/out: **yok**.

### 5.6 Inbox
- Hesap seçimi, konuşma listesi, mesaj geçmişi, text + template gönderimi, 24 saat oturum penceresi hem UI'da hem sunucuda kontrol ✓.
- Liste **N+1**: her chat için contact sorgusu + son inbound mesaj sorgusu; tüm chat'ler pagination olmadan yüklenir.
- Mesaj geçmişi pagination'sız tamamı döner; 5 sn polling.
- Atama, kapatma/açma, okunmamış sayısı, etiket/not, medya görüntüleme: **yok**.
- `assign_agent` otomasyon aksiyonu yalnızca sohbete sahte bir sistem mesajı yazar (gerçek atama yok).

### 5.7 Templates
- Meta'da oluşturma (component JSON, kategori MARKETING/UTILITY/AUTHENTICATION), Meta'dan silme, manuel sync (`updateOrCreate` by name+language) ✓.
- Template parametre eşleme (`getTemplateComponents`): BODY değişkenleri (pozisyonel ve named), URL button parametreleri. **HEADER parametreleri (text/medya) desteklenmez.**
- `SyncTemplateStatus` job'ı **hiçbir yerden dispatch edilmez** (ölü kod). Webhook'taki template durum event'i de işlenmez → onay durumu yalnızca manuel sync ile güncellenir.

### 5.8 Automations
- Tetikleyici: yalnızca gelen mesajda keyword (`exact`/`contains`, `*` = hepsi).
- Aksiyonlar (doğrulamada izin verilen 11 tip):

| Aksiyon | Durum |
|---|---|
| `send_message` | **Broken** (F2) |
| `add_to_group` / `remove_from_group` | Çalışıyor |
| `save_response` (notes/var1..5) | Çalışıyor |
| `condition` (contains/equals/starts_with/ends_with, AND/OR, dallanma) | Çalışıyor |
| `http_request` | Çalışıyor — SSRF koruması yok, timeout yok, imza yok |
| `send_email` | Çalışıyor (mail yapılandırılmışsa) |
| `delay` | Webhook isteği içinde `sleep()`, **en fazla 10 sn** (doğrulama 86400'e izin veriyor) |
| `assign_agent` | Partial — sahte sistem mesajı |
| `google_sheets` | **Missing** — boş placeholder |
| `disable_autoreply` | **Missing** — doğrulamada var, executor'da yok |

- Graf yürütmede `visited` seti ile döngü koruması var ✓; ama tekrar gelen webhook'ta yeniden çalışma koruması yok (F3).
- Import/export JSON formatı (`version: 1, type: zeromsg.automation_flow`) — Messora'da **uyumlu import** desteklenecek.

### 5.9 Developer API & giden webhook'lar
- API key: `zm_live_` + 40 hex, SHA-256 hash ile saklanır ✓, tek seferlik gösterim ✓, `last_used_at` ✓.
- API key'in `?api_key=` query parametresi ile de kabul edilmesi → loglara/proxy'lere sızma riski.
- Endpoint'ler: send-text (24h kontrolü yok), send-template. Rate limit yok, idempotency key yok.
- Giden webhook: yalnızca `message.received` event'i; HMAC-SHA256 imza ✓; 3 deneme; teslim logu yok; SSRF koruması yok; secret UI'da düz metin listelenir.

### 5.10 Dashboard & bildirimler
- Dashboard istatistikleri (aktif numara, kampanya, gönderilen/teslim/başarısız/alınan, haftalık aktivite, teslim durumu) ✓ — `whereIn(chat_ids)` ile tüm chat ID'lerini çeker, büyük veri setinde ağır.
- `system_notifications`: kampanya hatası, numara kalitesi düşüşü → toast + okunma işaretleme ✓.

### 5.11 Ayarlar
- Profil güncelleme/silme, parola değiştirme (6/dk), 2FA yönetimi, passkey yönetimi, tema (light/dark/system) ✓.
- İşletme/sistem ayarları ekranı yok (tüm yapılandırma `.env`).

### 5.12 Dosya ve medya
- Medya yönetimi **yok**. Yüklenen dosyalar yalnızca CSV/JSON parse için kullanılır, saklanmaz.

### 5.13 Testler
- 73 Pest testi: auth (Fortify standart), sayfa render smoke testleri, otomasyon CRUD/import/export, kampanya planlama ve direct mesaj, developer API (Http::fake), inbox oturum penceresi, dashboard sayaçları.
- **Kapsanmayanlar:** webhook imza/idempotency, durum sırası, resume, retry, eşzamanlılık, otomasyonun gerçekten mesaj göndermesi, tenant izolasyonu, import edge case'leri.

---

## 6. Doğrulanmış bulgular özeti

| ID | Bulgu | Önem | Kanıt |
|---|---|---|---|
| F1 | Duraklatılan kampanya devam ettirilemiyor | Yüksek | probe ✓ |
| F2 | Otomasyon `send_message` → `ArgumentCountError`, webhook 500 | Yüksek | probe ✓ |
| F3 | Tekrar gelen webhook otomasyonları/HTTP çağrılarını yeniden çalıştırıyor | Yüksek | probe ✓ |
| F4 | Webhook verify `hub.verify_token` kontrol etmiyor | Orta | probe ✓ |
| F5 | Sıra dışı durum webhook'u `read` → `delivered` geri düşürüyor | Orta | probe ✓ |
| F6 | `access_token`/`app_secret` DB'de düz metin ve tarayıcıya gönderiliyor | **Kritik** | probe ✓ |
| F7 | Başka tenant'ın grubuna contact bağlanabiliyor (IDOR); eksik opsiyonel alan → 500 | Orta | probe ✓ |
| F8 | `contact-groups.index` olmayan bir sayfa bileşenini render ediyor | Düşük | probe ✓ |
| F9 | Direct kampanya 24 saat penceresi kontrolü olmadan serbest metin gönderiyor | Yüksek (politika) | probe ✓ |
| F10 | Gönderim hatası/timeout = retry olmadan kalıcı `failed`; belirsiz durum yok | Yüksek | probe ✓ |
| F11 | Worker çökmesinde duplicate gönderim riski (kilit/lease yok) | Yüksek | kod analizi |
| F12 | Kampanya alıcıları/contact import HTTP isteği içinde satır satır | Yüksek (ölçek) | kod analizi |
| F13 | Graph API v21.0 varsayılanı 2027-01-21'de sona eriyor; sabit sürümlü URL'ler | Orta | resmî sürüm tablosu |
| F14 | Açık kayıt + otomatik tenant | Yüksek (self-host) | kod analizi |
| F15 | Opt-in/opt-out hiç yok | **Kritik** (uyumluluk) | kod analizi |
| F16 | `package-lock.json` senkron değil, `npm ci` başarısız | Düşük | komut çıktısı |
| F17 | Excel import kabul edilip CSV olarak parse ediliyor | Orta | kod analizi |
| F18 | `SyncTemplateStatus` ölü kod; template durum webhook'u işlenmiyor | Orta | kod analizi |
| F19 | `verify` endpoint'i sabit `000000` PIN ile numara register ediyor; `request-code` aslında kod istemiyor | Orta | kod analizi |
| F20 | Otomasyon HTTP aksiyonu ve giden webhook'larda SSRF koruması yok | Yüksek | kod analizi |

---

## 7. Korunması gereken değerler

Messora'ya taşınırken kaybedilmemesi gereken, ZeroMsg'de **gerçekten çalışan** işlevler:

1. UI tasarımı ve bileşen kütüphanesi (shadcn/Radix, Tailwind v4, dark mode, sidebar).
2. Otomasyon görsel akış editörü (`flow-builder.tsx`, 2.915 satır) ve JSON import/export formatı.
3. Template oluşturma sihirbazı (`templates/create.tsx`) ve parametre eşleme mantığı (BODY named/pozisyonel, URL button).
4. Kampanya oluşturma ekranı (grup veya CSV, değişken eşleme, planlama + timezone).
5. Inbox 24 saat pencere göstergesi ve template ile oturum açma.
6. Planlanmış kampanyaların atomik claim deseni.
7. API key'lerin hash'li saklanması ve tek seferlik gösterimi.
8. 2FA (TOTP) ve passkey desteği.
9. Webhook imza doğrulama (ham body + sabit zamanlı karşılaştırma).
10. Numara kalite düşüşü bildirimi.
