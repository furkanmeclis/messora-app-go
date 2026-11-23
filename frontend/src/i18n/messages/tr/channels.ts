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
            'Başarılı bir eşitleme yapılana veya yeni kimlik bilgileri girilene kadar gönderimler hemen başarısız olur.',
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
        open: 'Düzenle / yenile',
        title: '{name} hesabını düzenle',
        description:
            'Mevcut gizli bilgiyi korumak için alanı boş bırakın. Yeni belirteç önce Meta ile doğrulanır.',
        name: 'Ad',
        accessToken: 'Yeni erişim belirteci',
        appSecret: 'Yeni uygulama gizli anahtarı',
        secretConfigured: 'Uygulama gizli anahtarı tanımlı.',
        noSecret:
            "Henüz uygulama gizli anahtarı yok: eklenene kadar webhook'lar reddedilir.",
        disabled: 'Hesap devre dışı bırakıldı.',
        enabled: 'Hesap etkinleştirildi.',
        credentialsUpdated: 'Kimlik bilgileri güncellendi.',
        updated: 'Hesap güncellendi.',
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
