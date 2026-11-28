import type { Messages } from '../en';

const platform: Messages['platform'] = {
    name: 'Platform yönetimi',
    nav: {
        techProvider: 'Meta Tech Provider',
        overview: 'Genel bakış',
        tenants: 'Firmalar',
        users: 'Hesaplar',
        support: 'Destek oturumları',
        settings: 'Platform ayarları',
        metaRates: 'Meta fiyat kartı',
        aiPrices: 'AI fiyatları',
        audit: 'Denetim kaydı',
        signOut: 'Çıkış yap',
        catalogue: 'Katalog',
        operations: 'Operasyon',
    },
    login: {
        metaTitle: 'Platform yönetimi girişi',
        title: 'Platform yönetimi',
        description:
            'Messora firmalarını ve ayarlarını yönetmek için giriş yapın.',
        submit: 'Giriş yap',
        verifyTitle: 'İki adımlı doğrulama',
        verifyDescription:
            'Doğrulama uygulamanızdaki 6 haneli kodu ya da bir kurtarma kodunu girin.',
        verifySubmit: 'Doğrula',
        useRecovery: 'Kurtarma kodu kullan',
        useApp: 'Doğrulama uygulamasını kullan',
        bootstrapHint:
            'Henüz platform yöneticisi yok. `messora migrate` çıktısındaki bağlantıyı açın ya da `messora admin create` komutunu çalıştırın.',
    },
    bootstrap: {
        metaTitle: 'Platform yöneticisini oluştur',
        title: 'İlk platform yöneticisini oluşturun',
        description:
            'Bu tek kullanımlık bağlantı platformu yönetecek hesabı oluşturur. Ardından iki adımlı doğrulama kurulur.',
        submit: 'Yöneticiyi oluştur',
        missing:
            'Bu bağlantı eksik. Migrasyon günlüğündeki bağlantının tamamını kullanın.',
        done: 'Zaten bir platform yöneticisi var. Giriş yapın.',
    },
    enroll: {
        metaTitle: 'İki adımlı doğrulamayı kurun',
        title: 'İki adımlı doğrulamayı kurun',
        description:
            'Platform yöneticileri doğrulama uygulaması kullanmak zorundadır. Kodu okutun, ardından uygulamanın gösterdiği 6 haneli kodu girin.',
        start: 'QR kodu göster',
        secret: 'Ya da bu anahtarı girin: {secret}',
        code: 'Uygulamadaki kod',
        confirm: 'Aç',
        codesTitle: 'Kurtarma kodlarınızı saklayın',
        codesBody:
            'Telefonunuzu kaybederseniz her kod bir kez giriş yapmanızı sağlar. Yalnızca şimdi gösterilir.',
        continue: 'Panele devam et',
    },
    overview: {
        title: 'Genel bakış',
        description: 'Platformun özeti.',
        tenants: 'Firmalar',
        active: 'Etkin',
        suspended: 'Askıda',
        trial: 'Deneme sürümünde',
        users: 'Hesaplar',
        signups: 'Son 7 günde yeni',
        support: 'Açık destek oturumu',
    },
    tenants: {
        title: 'Firmalar',
        description: 'Platformdaki tüm çalışma alanları, en yeniden eskiye.',
        search: 'Ad, adres ya da alan adıyla ara',
        allStatuses: 'Tüm durumlar',
        create: 'Yeni firma',
        empty: 'Eşleşen firma yok.',
        more: 'Daha fazla',
        columns: {
            name: 'Firma',
            address: 'Adres',
            status: 'Durum',
            subscription: 'Abonelik',
            members: 'Üye',
            created: 'Oluşturulma',
        },
        status: {
            active: 'Etkin',
            suspended: 'Askıda',
            deleted: 'Silindi',
        },
        subscription: {
            trial: 'Deneme',
            active: 'Etkin',
            past_due: 'Ödeme gecikti',
            canceled: 'İptal',
        },
        trialEnds: 'Deneme {date} tarihinde biter',
        createTitle: 'Yeni firma',
        createDescription:
            'Çalışma alanını kendi alt alan adında açar. Sahip adresi verilirse davet hemen gönderilir.',
        name: 'Firma adı',
        slug: 'Adres',
        ownerEmail: 'Sahibin e-postası (isteğe bağlı)',
        trialDays: 'Deneme günü',
        trialDaysHint: '0 deneme olmadan başlatır.',
        language: 'Çalışma alanının dili',
        created: 'Firma oluşturuldu.',
        acceptLink: 'Davet bağlantısı (bir kez gösterilir)',
        acceptLinkHint:
            'E-posta yapılandırıldıysa gönderildi. Değilse bağlantıyı paylaşın.',
        copy: 'Kopyala',
        copied: 'Kopyalandı.',
    },
    tenant: {
        back: 'Tüm firmalar',
        open: 'Paneli aç',
        details: 'Ayrıntılar',
        rename: 'Yeniden adlandır',
        save: 'Kaydet',
        saved: 'Kaydedildi.',
        trialEnd: 'Deneme bitişi',
        clearTrial: 'Temizle',
        noTrial: 'Deneme bitişi yok',
        subscription: 'Abonelik',
        suspend: 'Askıya al',
        unsuspend: 'Askıyı kaldır',
        suspendTitle: '{name} askıya alınsın mı?',
        suspendBody:
            'Askıyı kaldırana kadar firmanın tüm adresleri "askıda" yanıtı verir. Verileri korunur.',
        reason: 'Neden',
        suspended: 'Askıda: {reason}',
        unsuspended: 'Askı kaldırıldı.',
        owners: 'Sahipler',
        noOwners: 'Henüz sahip yok.',
        inviteOwner: 'Sahip davet et',
        inviteTitle: 'Bir sahip davet edin',
        invited: 'Davet oluşturuldu.',
        domains: 'Alan adları',
        addDomain: 'Özel alan adı ekle',
        domainName: 'Alan adı',
        domainHint: 'örn. panel.ornek.com',
        verify: 'Şimdi doğrula',
        makePrimary: 'Ana adres yap',
        remove: 'Kaldır',
        removeTitle: '{domain} kaldırılsın mı?',
        removeBody: 'Firma bu adreste yanıt vermeyi bırakır.',
        primary: 'Ana',
        verified: 'Doğrulandı',
        pending: 'DNS bekleniyor',
        subdomain: 'Alt alan adı',
        custom: 'Özel',
        txtHint:
            'Bu TXT kaydını alan adının DNS sağlayıcısına ekleyin, ardından "Şimdi doğrula"ya basın:',
        support: 'Destek erişimi',
        supportTitle: 'Firmanın panelini açın',
        supportBody:
            'Süreli bir oturum; firmanın denetim kaydına yazılır ve süre boyunca panelinde bir uyarı görünür.',
        supportReason: 'Neden (firmaya gösterilir)',
        supportMinutes: 'Dakika',
        supportWrite: 'Değişikliğe izin ver (aksi halde salt okunur)',
        supportStart: 'Paneli aç',
        supportStarted: 'Destek oturumu yeni sekmede açıldı.',
        members: '{count} üye|{count} üye',
    },
    users: {
        title: 'Hesaplar',
        description: 'Tüm firmalardaki kişiler.',
        search: 'E-posta ya da adla ara',
        empty: 'Eşleşen hesap yok.',
        columns: {
            name: 'Ad',
            email: 'E-posta',
            status: 'Durum',
            memberships: 'Firmalar',
            lastLogin: 'Son giriş',
        },
        admin: 'Platform yöneticisi',
        unverified: 'Doğrulanmadı',
        resetRequired: 'Yeni şifre gerekli',
        mfa: '2FA',
        locked: 'Kilitli',
        active: 'Etkin',
        disabled: 'Devre dışı',
        never: 'Hiç',
    },
    user: {
        back: 'Tüm hesaplar',
        memberships: 'Üyelikler',
        none: 'Hiçbir firmanın üyesi değil.',
        disable: 'Hesabı devre dışı bırak',
        enable: 'Hesabı etkinleştir',
        disableTitle: '{email} devre dışı bırakılsın mı?',
        disableBody:
            'Kişinin tüm oturumları kapanır ve hesabı etkinleştirene kadar hiçbir firmaya giriş yapamaz.',
        forceReset: 'Yeni şifre iste',
        forceResetTitle: 'Yeni şifre istensin mi?',
        forceResetBody:
            'Tüm oturumlar kapanır ve kişiye bir sıfırlama bağlantısı gider. Yeni şifre belirlenince giriş yeniden çalışır.',
        done: 'Tamam.',
    },
    support: {
        title: 'Destek oturumları',
        description:
            'Bir platform yöneticisinin bir firmanın panelini açtığı her zaman.',
        empty: 'Henüz destek oturumu yok.',
        columns: {
            company: 'Firma',
            admin: 'Yönetici',
            reason: 'Neden',
            access: 'Erişim',
            window: 'Süre',
        },
        end: 'Şimdi bitir',
        ended: 'Bitirildi.',
        active: 'Açık',
        readOnly: 'Salt okunur',
        write: 'Değişikliğe izinli',
    },
    settings: {
        title: 'Platform ayarları',
        description: 'Bütün platform için geçerli ayarlar.',
        signup: 'Kendi kendine kayıt',
        signupHint:
            'Herkes platform adresinde bir firma açabilir (14 günlük deneme).',
        addresses: 'Adresler',
        baseDomain: 'Temel alan adı',
        adminUrl: 'Yönetim paneli',
        platformUrl: 'Kayıt adresi',
        email: 'Gönderici',
        emailHint:
            'Platform e-postaları kurulumun SMTP ayarlarıyla gider (üretimde Resend).',
        configured: 'Yapılandırıldı',
        notConfigured: 'Yapılandırılmadı',
        from: 'Gönderen',
        host: 'Sunucu',
        replyTo: 'Yanıt adresi',
        replyToHint:
            'Platform e-postalarına gelen yanıtların adresi (isteğe bağlı).',
        saved: 'Ayarlar kaydedildi.',
        notSet: 'Ayarlanmadı',
    },
    metaRates: {
        title: 'Meta fiyat kartı',
        description:
            'Bütün platform için tek fiyat kartı; her firma mesaj maliyetini bununla hesaplar.',
    },
    aiPrices: {
        title: 'AI fiyatları',
        description:
            'Bütün platform için tek fiyat tablosu; her firmanın AI maliyeti bundan hesaplanır.',
    },
    audit: {
        title: 'Denetim kaydı',
        description: 'Platform yöneticilerinin yaptıkları, en yeniden eskiye.',
        empty: 'Kayıt yok.',
        columns: {
            when: 'Zaman',
            who: 'Kim',
            action: 'İşlem',
            company: 'Firma',
            details: 'Ayrıntılar',
        },
        more: 'Daha fazla',
    },
    techProvider: {
        title: 'Meta Tech Provider',
        description:
            "Firmaların Embedded Signup ile (“Meta ile bağlan”) WhatsApp'ı bağladığı, Messora'nın kendi Meta uygulaması.",
        switch: 'Meta ile bağlan',
        switchHint:
            "Açıkken bütün firmalar “Meta ile bağlan”ı görür. Yalnızca Meta, Messora'yı Tech Provider olarak onayladıktan sonra açın. Kapatmak bu yolla bağlı firmaların olaylarını da durdurur.",
        ready: 'Hazır',
        incomplete: 'Eksik',
        app: "Messora'nın Meta uygulaması",
        appHint:
            "App ID ve App secret (App settings → Basic), Embedded Signup yapılandırması (Facebook Login for Business → Configurations) ve Messora işletmesinin bir system user token'ı. Sırlar şifreli saklanır, hiç gösterilmez.",
        appId: 'App ID',
        configurationId: 'Embedded Signup yapılandırma kimliği',
        appSecret: 'App secret',
        systemToken: "System user token'ı",
        secretPlaceholder: 'Değiştirmemek için boş bırakın',
        version: 'Graph API sürümü',
        scopes: 'App Review şu izinleri vermeli: {scopes}.',
        saved: 'Tech Provider ayarları kaydedildi.',
        webhook: 'Webhook',
        webhookHint:
            'Bunları uygulamanın WhatsApp → Configuration sayfasına girin ve alanlara (en az messages) abone olun. Messora üzerinden bağlanan bütün firmalar bu tek adresi kullanır.',
        callbackUrl: 'Geri çağırma adresi',
        verifyToken: 'Doğrulama belirteci',
        rotate: 'Yeni doğrulama belirteci',
        unrouted:
            'API açıldığından beri bilinmeyen hesaplara gelen olay: {count}',
        test: 'Meta ile test et',
        testHint:
            "Uygulama bilgilerini ve system user token'ını (debug_token), yapılandırma kimliğini ve uygulamanın webhook aboneliğini kontrol eder.",
        testButton: 'Test et',
        keys: {
            app: 'Uygulama',
            configuration: 'Yapılandırma',
            system_user_token: "System user token'ı",
            webhook: 'Webhook',
        },
        codes: {
            tp_app_ok: "Meta, App ID ve App secret'ı kabul etti.",
            tp_app_missing:
                "Messora'nın Meta uygulamasının App ID ve App secret'ını girin.",
            tp_app_invalid: 'Meta uygulama bilgilerini reddetti: {detail}',
            tp_configuration_ok:
                'Bir Embedded Signup yapılandırma kimliği ayarlı.',
            tp_configuration_missing:
                'Bir Embedded Signup yapılandırması oluşturup kimliğini girin.',
            tp_system_token_ok:
                "System user token'ı geçerli, bütün izinler var.",
            tp_system_token_missing:
                "Messora işletmesinin system user token'ı kayıtlı değil.",
            tp_system_token_invalid:
                "Meta, system user token'ının geçersiz olduğunu söylüyor: {detail}",
            tp_system_token_other_app:
                "System user token'ı {token_app} uygulamasına ait, {app_id} değil.",
            tp_system_token_scopes:
                "System user token'ında eksik izin: {missing}.",
            tp_subscription_ok:
                "Uygulamanın webhook'u Messora'nın Tech Provider adresine yönleniyor.",
            tp_subscription_missing:
                'Uygulamada WhatsApp Business hesapları için webhook yok; {callback_url} adresini doğrulama belirteciyle girin.',
            tp_subscription_url:
                "Uygulamanın webhook'u {current} adresine yönleniyor, {callback_url} değil.",
            tp_subscription_inactive:
                "Uygulamanın WhatsApp webhook'u etkin değil.",
            tp_subscription_fields:
                "Uygulamanın webhook'u messages alanına abone değil.",
            tp_subscription_error:
                'Meta uygulamanın webhook aboneliklerini vermedi: {detail}',
        },
    },
};
export default platform;
