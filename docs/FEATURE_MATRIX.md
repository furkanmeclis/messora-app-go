# Özellik Matrisi — ZeroMsg → Messora

> Phase 0 · 2026-11-22 · Kaynak commit `ac876d4`
> Durum ZeroMsg'in **kaynak kodundaki gerçek** durumunu gösterir; README esas alınmamıştır. Kanıtlar için bkz. [`ANALYSIS.md`](ANALYSIS.md) (bulgu ID'leri F1–F20).

**Durumlar:** `Implemented` kaynak kodda çalışır · `Partial` kısmen · `Missing` yok · `Broken` mevcut ama çalışmıyor · `Needs verification` koddan/testten doğrulanamadı

**Messora sütunu:** `Keep` davranışı koru · `Fix` koru + düzelt · `New` yeni geliştir · `Drop` bilinçli olarak alma · `Later` roadmap'e ertele — ve hedef phase (P1–P10).

## Kimlik doğrulama ve kullanıcılar

| Özellik | ZeroMsg | Kanıt | Messora |
|---|---|---|---|
| Login / logout | Implemented | Fortify, AuthenticationTest | Keep · P2 |
| Login rate limit (5/dk email+IP) | Implemented | FortifyServiceProvider | Keep + Redis tabanlı · P2 |
| Açık kayıt (register) | Implemented | RegistrationTest | **Drop** — yerine ilk kurulum akışı + davet (F14) · P2 |
| Şifre sıfırlama | Implemented | PasswordResetTest | Keep (SMTP gerekir) · P2 |
| E-posta doğrulama | Implemented | EmailVerificationTest | Davet kabulü ile birleşir · P2 |
| TOTP 2FA | Implemented | TwoFactorChallengeTest | Keep · P2 |
| Passkey (WebAuthn) | Implemented | Fortify passkeys | Later · P10 (go-webauthn) |
| Parola onayı (hassas sayfalar) | Implemented | PasswordConfirmationTest | Keep (re-auth) · P2 |
| Parola hash | Implemented (bcrypt) | Laravel | Fix → **Argon2id** · P2 |
| Profil güncelle / hesap sil | Implemented | ProfileUpdateTest | Keep · P2 |
| Tema (light/dark/system) | Implemented | appearance.tsx | Keep · P9 |
| Çoklu panel kullanıcısı (aynı işletme) | Missing | `users()->first()` | New · P2 |
| Roller (Owner/Admin/Manager/Agent) | Missing | `role='client'` | New · P2 |
| İzin tabanlı yetkilendirme | Missing | — | New · P2 |
| Oturum yönetimi (aktif oturumları listele/iptal) | Missing | — | New · P2 |
| Audit log | Missing | — | New · P2 |
| Multi-tenant workspace | Implemented | tenants | **Drop** (tek işletme kararı) |

## WhatsApp hesabı ve Cloud API

| Özellik | ZeroMsg | Kanıt | Messora |
|---|---|---|---|
| Manuel bağlantı (WABA ID, Phone Number ID, token) | Implemented | `requestCode` (isim yanıltıcı) | Fix · P3 |
| Embedded Signup | Needs verification | Kod var, test yok | Later (talimat: zorunlu değil) |
| WABA'dan numara sync | Implemented | `sync` | Keep · P3 |
| Numara doğrulama / register | Broken | sabit `000000` PIN, kod istenmiyor (F19) | Fix (kullanıcı PIN'i) · P3 |
| Numara kalite/limit bilgisi | Implemented | `getPhoneNumberDetails` | Keep · P3 |
| Kalite düşüş webhook'u → bildirim | Implemented | `handlePhoneNumberQualityUpdate` | Keep · P3 |
| Token/secret şifreli saklama | Missing | düz metin, tarayıcıya gidiyor (F6) | New (AES-256-GCM) · P3 |
| Graph API sürümü yapılandırılabilir | Partial | env var, ama sabit URL'ler var (F13) | Fix → varsayılan v26.0 · P3 |
| Business profile okuma | Partial | client metodu var, UI yok | Keep · P3 |
| Çoklu numara veri modeli | Implemented | `whatsapp_accounts` | Keep (account + phone_numbers ayrımı) · P3 |

## Mesaj tipleri

| Özellik | ZeroMsg | Messora |
|---|---|---|
| Text gönderim | Implemented | Keep · P3 |
| Template gönderim (BODY + URL button param) | Implemented | Keep · P3 |
| Template HEADER parametreleri (text/medya) | Missing | New · P3 |
| Image / video / document / audio gönderim | Missing | New · P3 |
| Location gönderim | Missing | New · P3 |
| Interactive (button / list) gönderim | Missing | New · P3 |
| Inbound text / button | Implemented | Keep · P3 |
| Inbound medya / location / interactive / reaction | Missing (placeholder metin) | New · P3/P7 |
| Medya indirme (Meta → storage) | Missing | New · P3 |
| Medya yükleme (storage → Meta) | Missing | New · P3 |
| `biz_opaque_callback_data` korelasyonu | Missing | New · P5 (belirsiz gönderim uzlaştırma) |

## Webhook

| Özellik | ZeroMsg | Kanıt | Messora |
|---|---|---|---|
| Verify challenge | Broken | `hub.verify_token` kontrol edilmiyor (F4) | Fix · P3 |
| İmza doğrulama (X-Hub-Signature-256) | Implemented | `isValidSignature` | Keep · P3 |
| Ham event kalıcı kaydı | Missing | — | New · P3 |
| Asenkron işleme (önce kaydet, sonra işle) | Missing | senkron | New · P3 |
| Mesaj idempotency | Partial | mesaj tekil, yan etkiler tekrar (F3) | Fix · P3 |
| Durum güncelleme (sent/delivered/read/failed) | Partial | sıra korunmuyor (F5), hata detayı yok | Fix · P3 |
| Durum geçmişi | Missing | — | New · P3 |
| Template durum webhook'u | Missing | işlenmiyor (F18) | New · P3 |

## Template yönetimi

| Özellik | ZeroMsg | Messora |
|---|---|---|
| Listeleme / detay | Implemented | Keep · P3 |
| Meta'da oluşturma (sihirbaz) | Implemented | Keep · P3 |
| Meta'dan silme | Implemented | Keep · P3 |
| Manuel senkronizasyon | Implemented | Keep · P3 |
| Otomatik periyodik senkronizasyon | Broken | `SyncTemplateStatus` hiç çağrılmıyor (F18) | Fix (scheduler) · P3 |

## Contacts, gruplar, segmentasyon

| Özellik | ZeroMsg | Kanıt | Messora |
|---|---|---|---|
| Contact CRUD | Implemented | ContactController | Keep · P4 |
| Arama + grup filtresi + pagination | Implemented | | Keep (pg_trgm) · P4 |
| Telefon normalizasyonu E.164 | Partial | yalnızca rakam temizleme | Fix (libphonenumber) · P4 |
| Duplicate önleme | Partial | import'ta var, manuel eklemede yok | Fix (unique constraint) · P4 |
| Özel alanlar | Partial | sabit `var1..var5` | Fix (tanımlı custom field'lar; var1–5 geriye uyumlu eşleme) · P4 |
| Gruplar + üyelik | Implemented | | Keep · P4 |
| Grup yönetim sayfası | Broken | sayfa bileşeni yok (F8) | Fix · P9 |
| Etiketler | Missing | | New · P4 |
| Notlar (çoklu, yazar bilgili) | Partial | tek `notes` alanı | New · P4 |
| İzin durumu (opt-in/opt-out) | Missing | (F15) | New · P4 |
| Son mesaj tarihi | Missing | yalnızca chat'te | New · P4 |
| CSV import | Partial | senkron, sabit başlık, rapor yok (F12) | Fix (worker, kolon eşleme, hata raporu, progress, iptal) · P4 |
| Excel import | Broken | xlsx kabul edilip CSV parse (F17) | Fix · P4 |
| CSV export | Implemented | belleğe tümünü alır | Keep + streaming · P4 |
| Excel export | Missing | | New · P4 |
| Dinamik segmentler | Missing | | New · P4 |
| Tenant/yetki kontrolsüz ID kabulü | Broken | IDOR (F7) | Fix · P4 |

## Kampanyalar

| Özellik | ZeroMsg | Kanıt | Messora |
|---|---|---|---|
| Draft kampanya | Implemented | | Keep · P5 |
| Planlanmış kampanya + timezone | Implemented | CampaignSchedulingTest | Keep · P5 |
| Başlatma | Implemented | | Keep (async) · P5 |
| Duraklatma | Implemented | | Keep · P5 |
| Devam ettirme | Broken | (F1) | Fix · P5 |
| İptal | Missing | | New · P5 |
| Alıcı kaynağı: grup | Implemented | | Keep · P5 |
| Alıcı kaynağı: CSV yükleme | Implemented | | Keep · P5 |
| Alıcı kaynağı: segment | Missing | | New · P5 |
| Alıcı dedupe | Missing | | New · P5 |
| Opt-out'lara gönderim engeli | Missing | (F15) | New · P5 (sistem seviyesinde) |
| Direct (serbest metin) kampanya | Broken (politika) | 24h kontrolü yok (F9) | Fix: yalnızca açık oturumu olan alıcılara; diğerleri `skipped_no_session` · P5 |
| Rate limiting | Partial | sabit batch gecikmesi; mps config kullanılmıyor | Fix (numara başına token bucket + 130429 backoff) · P5 |
| Retry + exponential backoff | Broken | (F10) | Fix · P5 |
| Duplicate gönderim önleme | Missing | (F11) | New · P5 |
| Belirsiz (timeout) gönderim yönetimi | Missing | (F10) | New · P5 |
| Dead letter / kalıcı hata yönetimi | Missing | | New · P5 |
| Sayaçlar (sent/delivered/read/failed) | Partial | yarış durumu, O(n) recompute | Fix · P5 |
| Bekleyen sayısı | Partial | progress % var | Keep · P5 |
| Hata kayıtları | Partial | yalnızca exception mesajı | Fix (Meta hata kodu + detay) · P5 |
| Kampanya geçmişi / event log | Missing | | New · P5 |
| Maliyet takibi | Broken | `total_cost` hep 0 | Later (Meta pricing webhook alanları) · P10 |
| Büyük kampanya ölçeklenebilirliği | Broken | HTTP içinde insert, belleğe yükleme (F12) | Fix · P5 |

## Inbox

| Özellik | ZeroMsg | Messora |
|---|---|---|
| Konuşma listesi | Implemented (N+1, pagination yok) | Fix · P7 |
| Mesaj geçmişi | Implemented (pagination yok) | Fix (cursor) · P7 |
| Text yanıt + 24h pencere kontrolü | Implemented | Keep · P7 |
| Template ile yanıt | Implemented | Keep · P7 |
| Medya gönderme/görüntüleme | Missing | New · P7 |
| Okundu bilgisi (gelen mesajı okundu işaretle) | Missing | New · P7 |
| Mesaj durumları (tik) | Implemented | Keep · P7 |
| Konuşma atama | Missing (otomasyonda sahte) | New · P7 |
| Konuşma kapatma/açma | Missing | New · P7 |
| Müşteri bilgi paneli | Partial (isim/telefon) | Fix · P7 |
| Etiket/not yönetimi | Missing | New · P7 |
| Gerçek zamanlı güncelleme | Missing (5 sn polling) | New (Centrifugo) · P6/P7 |

## Otomasyon

| Özellik | ZeroMsg | Messora |
|---|---|---|
| Görsel akış editörü + dallanma | Implemented | Keep · P8/P9 |
| Import / export (JSON v1) | Implemented | Keep (v1 uyumlu import) · P8 |
| Tetikleyici: keyword (exact/contains/*) | Implemented | Keep · P8 |
| Tetikleyici: yeni mesaj | Implemented (`*`) | Keep · P8 |
| Tetikleyici: yeni contact / etiket eklendi / konuşma açıldı | Missing | New · P8 |
| Aksiyon: otomatik yanıt | Broken (F2) | Fix · P8 |
| Aksiyon: template gönder | Missing | New · P8 |
| Aksiyon: gruba ekle/çıkar | Implemented | Keep · P8 |
| Aksiyon: etiket ekle | Missing | New · P8 |
| Aksiyon: yanıtı alana kaydet | Implemented | Keep · P8 |
| Aksiyon: koşul | Implemented | Keep · P8 |
| Aksiyon: HTTP isteği / webhook | Implemented (SSRF açık, F20) | Fix · P8 |
| Aksiyon: e-posta | Implemented | Keep · P8 |
| Aksiyon: gecikme | Partial (sleep ≤10 sn, istek içinde) | Fix (zamanlanmış task) · P8 |
| Aksiyon: konuşma ata | Partial (sahte mesaj) | Fix · P8 |
| Aksiyon: konuşma kapat | Missing | New · P8 |
| Aksiyon: Google Sheets | Missing (placeholder) | Later — UI'dan kaldırılır, import'ta uyarı |
| Aksiyon: disable_autoreply | Missing (validator'da var) | New (konuşma bazında otomasyonu sustur) · P8 |
| Worker'da çalıştırma | Missing (webhook isteğinde) | New · P8 |
| Döngü / tekrar / duplicate aksiyon koruması | Partial (yalnızca graf içi visited) | Fix · P8 |
| Çalıştırma geçmişi | Missing | New · P8 |

## Geliştirici API ve entegrasyonlar

| Özellik | ZeroMsg | Messora |
|---|---|---|
| API key oluştur/iptal, hash'li saklama | Implemented | Keep (scope'lu) · P2 |
| `POST /api/v1/messages/send-text` | Implemented (24h kontrolü yok) | Fix · P3 |
| `POST /api/v1/messages/send-template` | Implemented | Keep · P3 |
| API key query parametresiyle | Implemented | **Drop** (yalnızca header) |
| API rate limit | Missing | New · P2 |
| Idempotency-Key | Missing | New · P2/P3 |
| OpenAPI dokümantasyonu | Missing | New · P2 |
| Giden webhook (message.received) | Implemented | Keep + yeni event'ler · P8 |
| Giden webhook imzası (HMAC) | Implemented | Keep · P8 |
| Giden webhook teslim logu / tekrar gönder | Missing | New · P8 |
| SSRF koruması | Missing (F20) | New · P8 |

## Gerçek zamanlı, bildirim, gözlemlenebilirlik

| Özellik | ZeroMsg | Messora |
|---|---|---|
| Uygulama içi bildirim + toast | Implemented | Keep · P6 |
| WebSocket gerçek zamanlı event | Missing | New (Centrifugo v6) · P6 |
| Dashboard istatistikleri | Implemented | Keep · P5/P9 |
| Yapılandırılmış log + request ID | Missing (Laravel default log) | New · P1 |
| Prometheus metrikleri | Missing | New · P1+ |
| Health endpoint | Partial (Laravel `/up`) | New (live/ready) · P1 |
| OpenTelemetry tracing | Missing | New (opsiyonel, OTLP) · P10 |

## Kurulum ve operasyon

| Özellik | ZeroMsg | Messora |
|---|---|---|
| Docker Compose kurulum | Missing (Sail dev only) | New · P1/P10 |
| Tekrarlanabilir frontend build | Broken (`npm ci` başarısız, F16) | Fix (lockfile CI kontrolü) · P1 |
| Otomatik installer | Implemented (`composer run zeromsg:install`) | Yerine: migrate servisi + ilk kurulum akışı · P1/P2 |
| Demo verisi | Implemented (sabit parola) | Yalnızca `APP_ENV=development` · P10 |
| Backup/restore dokümanı | Missing | New · P10 |
| i18n | Missing (UI İngilizce, bildirimler Arapça sabit) | Later (EN + TR) · P9+ |
