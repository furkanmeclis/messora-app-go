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
        appId: 'Uygulama ID (isteğe bağlı)',
        appSecret: 'Uygulama gizli anahtarı (önerilir)',
        appSecretHint:
            "Webhook imzalarını doğrulamak için gerekir. Girilmezse Meta'dan gelen tüm webhook'lar reddedilir.",
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
            "Uygulama gizli anahtarı tanımlı olmadığından Meta'dan gelen tüm webhook'lar reddediliyor: gelen mesaj ve teslim bilgisi alınmaz. Bir hesaba uygulama gizli anahtarını ekleyin (Düzenle / yenile) ya da META_APP_SECRET değişkenini ayarlayın.",
        callbackUrl: 'Geri çağırma adresi (Callback URL)',
        verifyToken: 'Doğrulama belirteci (Verify token)',
        copyCallbackUrl: 'Geri çağırma adresini kopyala',
        copyVerifyToken: 'Doğrulama belirtecini kopyala',
        copied: '{label} kopyalandı',
        copyFailed:
            'Burada kopyalamaya izin verilmiyor; metni seçip kopyalayın.',
        fields: 'Abone olunacak alanlar',
    },
};
export default channels;
