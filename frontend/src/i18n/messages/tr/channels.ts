import type { Messages } from '../en';

const channels: Messages['channels'] = {
    title: 'Kanallar',
    description:
        "WhatsApp Business hesapları, telefon numaraları ve Meta'nın çağırdığı webhook.",
    loading: 'Yükleniyor',
    accounts: {
        title: 'WhatsApp hesapları',
        description:
            'Erişim belirteçleri ve uygulama gizli anahtarları şifrelenerek saklanır ve bir daha gösterilmez.',
        label: 'WhatsApp hesapları',
        empty: 'Bağlı WhatsApp hesabı yok',
        emptyManage:
            'Mesaj, şablon ve kampanya göndermek için bir WhatsApp Business hesabı bağlayın.',
        emptyAsk:
            'Bir WhatsApp Business hesabı bağlaması için yöneticinize başvurun.',
        statuses: {
            active: 'Etkin',
            error: 'Hata',
            disabled: 'Devre dışı',
        },
        noAppSecret: 'Uygulama gizli anahtarı yok',
        wabaId: 'WABA ID',
        appId: 'Uygulama ID',
        lastSynced: 'Son eşitleme',
        connected: 'Bağlanma',
        sync: 'Eşitle',
        synced: '{numbers} telefon numarası ve {templates} şablon eşitlendi.',
        enable: 'Etkinleştir',
        disable: 'Devre dışı bırak',
        lastError: 'Son hata',
        lastErrorHint:
            'Bağlantı kontrolü başarılı olana veya token değiştirilene kadar gönderimler hemen başarısız olur.',
        viewTemplates: 'Şablonları görüntüle',
    },
    connect: {
        open: 'Hesap bağla',
        title: 'WhatsApp Business hesabı bağlayın',
        description:
            'whatsapp_business_messaging ve whatsapp_business_management izinlerine sahip, süresiz bir sistem kullanıcısı belirteci kullanın. Messora bir şey kaydetmeden önce belirteci Meta ile doğrular.',
        name: 'Ad',
        namePlaceholder: 'Ana marka',
        wabaId: 'WhatsApp Business Hesap ID',
        digitsOnly: 'Yalnızca rakam',
        wabaIdHint:
            'Yalnızca rakam; Meta Business Suite → WhatsApp hesapları bölümünde bulunur.',
        accessToken: 'Erişim belirteci',
        accessTokenHint: 'Şifrelenerek saklanır; bir daha gösterilmez.',
        appId: 'Uygulama ID (App ID)',
        appIdHint: 'Meta uygulamanız: App settings → Basic.',
        appSecret: 'Uygulama gizli anahtarı (App secret)',
        appSecretHint:
            "App ID ile aynı sayfada. Uygulamanızın gönderdiği her webhook'un imzasını doğrular; şifrelenerek saklanır, bir daha gösterilmez.",
        submit: 'Bağla',
        done: 'Hesap bağlandı. Telefon numaraları ve şablonlar içe aktarıldı.',
    },
    edit: {
        open: 'Düzenle / token değiştir',
        title: '{name} hesabını düzenle',
        description:
            'Mevcut gizli bilgiyi korumak için alanı boş bırakın. Yeni belirteç önce Meta ile doğrulanır.',
        name: 'Ad',
        accessToken: 'Yeni erişim belirteci',
        accessTokenHint:
            'Önce Meta ile kontrol edilir (geçerlilik, izinler, süre). Çalışan bir token hesabı hemen yeniden etkinleştirir.',
        appIdHint:
            "App ID ve gizli anahtar girilirse Messora token'ın bitiş tarihini okuyabilir ve kısa ömürlü kullanıcı token'larını 60 günlük token'a çevirir.",
        appSecret: 'Yeni uygulama gizli anahtarı',
        secretConfigured: 'Uygulama gizli anahtarı tanımlı.',
        noSecret:
            "Henüz uygulama gizli anahtarı yok: eklenene kadar webhook'lar reddedilir.",
        disabled: 'Hesap devre dışı bırakıldı.',
        enabled: 'Hesap etkinleştirildi.',
        credentialsUpdated: 'Kimlik bilgileri güncellendi.',
        updated: 'Hesap güncellendi.',
    },
    token: {
        permanent: 'Kalıcı token',
        until: '{date} tarihine kadar geçerli',
        expired: "Token'ın süresi doldu",
        unknown: 'Token kontrol edilmedi',
        systemUser: 'System User token',
        type: '{type} token',
        checkedAt: 'Kontrol: {date}',
        neverChecked: 'Hiç kontrol edilmedi',
        check: 'Bağlantıyı kontrol et',
        checkOk: 'Bağlantı çalışıyor.',
        checkFailed: "Meta token'ı reddetti; hesaba bakın.",
        replace: "Token'ı değiştir",
        problemTitle: 'Meta bu hesabı reddetti',
        problemHint:
            'Token değiştirilene kadar bu hesaptan gönderim yapılamaz. Sıradaki mesajlar bu sebeple başarısız olur.',
    },
    guide: {
        title: 'Süresi hiç dolmayan token alın',
        why: "Uygulama panelinden veya Graph Explorer'dan kopyalanan token'lar birkaç saat ya da gün içinde dolar ve hesap gönderimi durdurur. System User token'ının süresi dolmaz.",
        step1: 'Meta İşletme Ayarları → Kullanıcılar → Sistem kullanıcıları bölümünü açın ve Yönetici rolüyle bir sistem kullanıcısı ekleyin.',
        step2: 'Varlık atayın: WhatsApp hesabınız (Tam kontrol) ve Meta uygulamanız.',
        step3: '"Yeni token oluştur"a tıklayın, uygulamanızı seçin ve token süresini "Hiçbir zaman" yapın.',
        step4: 'whatsapp_business_messaging ve whatsapp_business_management izinlerini işaretleyin.',
        step5: "Token'ı kopyalayıp buraya yapıştırın. Messora kaydetmeden önce Meta ile kontrol eder.",
        open: 'İşletme Ayarları → Sistem kullanıcılarını aç',
    },
    disconnect: {
        open: 'Bağlantıyı kes',
        title: '{name} bağlantısı kesilsin mi?',
        blockedTitle: 'Bu hesap kaldırılamaz',
        description:
            "Hesabın telefon numaraları ve şablonları Messora'dan kaldırılır. Mesaj geçmişi olan hesaplar kaldırılamaz; bunun yerine devre dışı bırakın.",
        blockedDescription:
            '{reason} Konuşmalar ve teslim geçmişi bu hesabın numaralarına bağlı olduğundan hesap kalır; devre dışı bir hesap hiçbir şey göndermez ve daha sonra yeniden etkinleştirilebilir.',
        disableInstead: 'Bunun yerine devre dışı bırak',
        confirm: 'Bağlantıyı kes',
        done: 'Hesabın bağlantısı kesildi.',
    },
    numbers: {
        title: 'Telefon numaraları',
        description:
            'Bir istek veya kampanya gönderen belirtmediğinde varsayılan numara kullanılır.',
        empty: 'Henüz telefon numarası yok. Numaralar bir hesap bağlandığında veya eşitlendiğinde içe aktarılır.',
        number: 'Numara',
        verifiedName: 'Doğrulanmış ad',
        account: 'Hesap',
        quality: 'Kalite',
        messagingLimit: 'Mesajlaşma limiti',
        throughput: 'Hız: {level}',
        throughputMps: 'Gönderim hızı',
        perSecond: '{mps} mesaj/sn',
        signalsChecked: 'Meta’dan okundu: {when}',
        default: 'Varsayılan',
        setDefault: 'Varsayılan yap',
        defaultChanged: 'Varsayılan numara değiştirildi.',
        qualities: {
            unknown: 'Bilinmiyor',
            high: 'Yüksek',
            medium: 'Orta',
            low: 'Düşük',
        },
    },
    webhook: {
        title: 'Webhook kurulumu',
        description:
            'Bu değerleri Meta App panelinde WhatsApp → Configuration bölümüne girin. Geri çağırma adresine herkese açık HTTPS üzerinden erişilebilmelidir.',
        signatureOff: 'İmza doğrulaması kapalı',
        signatureOffHint:
            "Hiçbir hesapta uygulama gizli anahtarı olmadığından Meta'dan gelen tüm webhook'lar reddediliyor: gelen mesaj ve teslim bilgisi alınmaz. Bir hesaba uygulama gizli anahtarını ekleyin (Düzenle / token değiştir).",
        callbackUrl: 'Geri çağırma adresi (Callback URL)',
        verifyToken: 'Doğrulama belirteci (Verify token)',
        copyCallbackUrl: 'Geri çağırma adresini kopyala',
        copyVerifyToken: 'Doğrulama belirtecini kopyala',
        copied: '{label} kopyalandı',
        copyFailed:
            'Burada kopyalamaya izin verilmiyor; metni seçip kopyalayın.',
        fields: 'Abone olunacak alanlar',
        moved: 'Geri çağırma adresiniz, doğrulama belirteciniz, bağlantı testi ve webhook durumu Ayarlar → WhatsApp / Meta altında.',
        open: 'Meta bağlantısını aç',
    },
    meta: {
        tab: 'WhatsApp / Meta',
        title: 'WhatsApp / Meta bağlantısı',
        description:
            "Firmanızın kendi Meta uygulamasını bağlayın. Messora yalnızca sizin bilgilerinizi kullanır: Meta'nın webhook'ları size özel adrese gelir ve uygulama gizli anahtarınızla doğrulanır.",
        method: {
            title: 'Bağlantı yöntemi',
            ownApp: 'Kendi Meta uygulamanız',
            ownAppHint:
                "Bir Meta uygulaması oluşturup WhatsApp ürününü eklersiniz; App ID, App secret ve System User token'ını buraya girersiniz. Tam kontrol; şimdi kullanılabilir.",
            techProvider: 'Messora Tech Provider',
            soon: 'Yakında',
            techProviderHint:
                'Uygulama oluşturmadan, birkaç tıklamayla "Meta ile bağla". Meta, Messora\'yı Tech Provider olarak onayladığında açılacak.',
        },
        steps: {
            title: 'Kendi Meta uygulamanızı bağlayın',
            description: 'Adımları sırayla izleyin.',
            app: {
                title: 'Meta uygulaması oluşturun',
                body: 'Meta for Developers → My Apps → Create app ile Business türünde bir uygulama oluşturun ve WhatsApp ürününü ekleyin. App ID ve App secret, App settings → Basic sayfasındadır.',
                link: "Meta for Developers'ı aç",
            },
            token: {
                title: "System User token'ı oluşturun",
                body: 'Business Settings → System users bölümünde Admin rolünde bir sistem kullanıcısı ekleyin, ona uygulamanızı ve WhatsApp hesabınızı atayın; whatsapp_business_messaging ve whatsapp_business_management izinleriyle süresi dolmayan bir token üretin.',
            },
            account: {
                title: 'Bilgileri girin',
                body: "App ID, App secret, System User token'ı ve WhatsApp Business Hesap ID'niz. Messora bunları kaydetmeden önce Meta ile doğrular; gizli bilgiler şifrelenerek saklanır ve bir daha gösterilmez.",
                connect: 'Hesap bağla',
                another: 'Başka bir hesap bağla',
                manage: 'Bilgileri değiştirmek ve numaraları yönetmek için Kanallar sayfasına gidin.',
            },
            webhook: {
                title: "Uygulamanızda webhook'u ayarlayın",
                body: 'Uygulamanızda WhatsApp → Configuration → Webhook → Edit\'i açın, aşağıdaki geri çağırma adresini ve doğrulama belirtecini yapıştırıp "Verify and save"e tıklayın. Ardından şu alanlara abone olun:',
            },
            test: {
                title: 'Bağlantıyı test edin',
                body: "Messora token'ı, telefon numaralarını ve uygulamanızın WhatsApp hesabındaki webhook aboneliğini kontrol eder (eksikse abone eder), ardından webhook gelip gelmediğini gösterir.",
            },
        },
        hostUrl: 'Alternatif adres',
        hostUrlHint:
            "Çalışma alanınızın kendi adresi de aynı webhook'ları kabul eder (mevcut kurulumlar için). Yukarıdaki geri çağırma adresini tercih edin: alan adınız değişse de değişmez.",
        rotate: {
            open: 'Yeni adres',
            title: 'Yeni webhook adresi oluşturulsun mu?',
            description:
                'Adres sızdıysa kullanın. Yeni geri çağırma adresini Meta uygulamanıza girin; eski adres seçtiğiniz süre boyunca çalışmaya devam eder.',
            grace: 'Eski adres',
            graceNone: 'hemen kapansın',
            graceDay: '24 saat çalışsın',
            grace3: '3 gün çalışsın',
            graceWeek: '7 gün çalışsın',
            confirm: 'Yeni adres oluştur',
            done: 'Yeni webhook adresi oluşturuldu. Meta uygulamanıza girin.',
            previousUntil: 'Önceki adres {date} tarihine kadar çalışır.',
        },
        verify: {
            open: 'Yeni doğrulama belirteci',
            title: 'Doğrulama belirteci yenilensin mi?',
            description:
                'Meta bunu yalnızca geri çağırma adresi doğrulanırken ister. Eski belirteç hemen geçersiz olur; yeniden doğrularken yenisini girin.',
            confirm: 'Yenile',
            done: 'Doğrulama belirteci yenilendi.',
        },
        health: {
            title: 'Bağlantı durumu',
            description:
                "Uygulamanızın gönderdiği webhook'lardan Messora'nın gördükleri.",
            lastWebhook: 'Son webhook',
            never: 'Henüz yok',
            signatureFailures: 'Reddedilen imzalar',
            lastAt: 'son: {date}',
            mismatches: 'Bağlı olmayan hesaplara gelen olaylar',
            noSecret:
                "Henüz hiçbir hesapta uygulama gizli anahtarı yok; Meta'dan gelen tüm webhook'lar reddediliyor.",
            signatureHint:
                "Son webhook'ların imzası hatalıydı. App secret'ın, webhook'u buraya yönlendiren uygulamaya ait olduğunu kontrol edin.",
            mismatchHint:
                'Uygulamanız burada bağlı olmayan {waba} WhatsApp hesabı için olay gönderdi; bunlar işlenmedi. O hesabı bağlayın ya da uygulamanızda aboneliğini kaldırın.',
            tokens: "Erişim token'ları",
        },
        test: {
            button: 'Bağlantıyı test et',
            ok: 'Bağlantı çalışıyor.',
            failed: 'Test bir sorun buldu.',
            none: 'Önce bir hesap bağlayın; test hesap başına çalışır.',
            keys: {
                app_secret: 'Uygulama bilgileri',
                token: 'Erişim token’ı',
                phone_numbers: 'Telefon numaraları',
                subscription: 'Webhook aboneliği',
                webhook: "Gelen webhook'lar",
            },
            codes: {
                app_secret_missing:
                    "Uygulama gizli anahtarı kayıtlı değil; bu hesabın webhook'ları reddediliyor. Düzenle / token değiştir ile ekleyin.",
                app_id_missing:
                    'App ID kayıtlı değil; token uygulamanızla doğrulanamaz ve medyalı şablon oluşturulamaz.',
                app_ok: 'App ID ve App secret kayıtlı.',
                token_invalid: "Meta token'ı reddetti: {detail}",
                token_unchecked:
                    "Meta token'ın bitiş tarihini bildirmedi. App ID ile App secret'ın aynı uygulamaya ait olduğunu kontrol edin ya da System User token'ı kullanın.",
                token_other_app:
                    "Token {token_app} uygulamasına ait, {app_id} App ID'sine değil. Aynı uygulamanın System User token'ını kullanın.",
                token_expires:
                    "Token'ın süresi {expires_at} tarihinde doluyor. Süresi dolmayan bir System User token'ıyla değiştirin.",
                token_ok: 'Geçerli ve süresi dolmuyor.',
                numbers_error:
                    'Meta telefon numaralarını listelemeyi reddetti: {detail}',
                numbers_none:
                    'WhatsApp hesabında henüz telefon numarası yok; WhatsApp Manager’dan ekleyin.',
                numbers_ok: '{count} telefon numarası bulundu.',
                subscription_error:
                    'Webhook aboneliği okunamadı ya da oluşturulamadı: {detail}',
                subscribed_now:
                    "Uygulamanız hesabın webhook'larına abone değildi; Messora şimdi abone etti.",
                subscribed: "Uygulamanız hesabın webhook'larına abone.",
                not_subscribed:
                    "Uygulamanız hâlâ hesabın webhook'larına abone değil.",
                webhook_none:
                    'Henüz webhook gelmedi. Uygulamanızdaki geri çağırma adresini, doğrulama belirtecini ve abone olunan alanları kontrol edin; ilk olay bir sonraki mesaj ya da durum bildirimiyle gelir.',
                webhook_unknown: 'Webhook durumu okunamadı.',
                webhook_signature:
                    "Webhook'lar geliyor ama sonuncusunun ({at}) imzası hatalıydı. App secret'ı kontrol edin.",
                webhook_ok: 'Son webhook {at} geldi.',
            },
        },
    },
};
export default channels;
