import type { Messages } from '../en';

const settings: Messages['settings'] = {
    title: 'Ayarlar',
    description: 'Çalışma alanının genel yapılandırması.',
    tabs: {
        general: 'Genel',
        email: 'E-posta',
        security: 'Güvenlik',
        webhooks: "Webhook'lar",
        metaRates: 'Meta fiyat kartı',
    },
    metaRates: {
        managedByPlatform:
            'Meta’nın fiyat kartını tüm çalışma alanları için Messora güncel tutar; mesaj maliyetleri buradan hesaplanır.',
        title: 'Meta fiyat kartı',
        description:
            'Meta mesaj başı fiyatlarını API ile değil, yalnızca fiyat kartı olarak yayımlar. Kartı buraya aktarın; ücretli her mesaj hesabınızın para biriminde fiyatlandırılır. Meta’nın kendi günlük toplamları karşılaştırma için ayrıca çekilir.',
        hint: 'En fazla 2 MB CSV dosyası. Aynı para birimi, pazar, kategori, kademe ve tarihteki satırlar öncekinin yerini alır.',
        format: 'Dosya biçimi',
        formatHelp:
            'Sütunlar: market (pazar), countries (boşluk veya ; ile ayrılmış ISO kodları), category (marketing, utility, authentication, authentication-international, service), price (fiyat), currency (para birimi), effective_from (YYYY-AA-GG), isteğe bağlı tier_lower ve tier_upper (ay içindeki mesaj sırası) ve source. Fiyatları Meta’nın yayımladığı fiyat kartından alın; aşağıdaki değerler yalnızca örnektir.',
        imported: '{markets} ülke için {rates} fiyat aktarıldı.',
        importFailed: 'Fiyat kartı aktarılamadı.',
        empty: 'Henüz fiyat yok. Fiyat kartı aktarılana kadar mesaj maliyetleri hesaplanmaz.',
        market: 'Pazar',
        category: 'Kategori',
        price: 'Mesaj başı fiyat',
        tier: 'Aylık kademe',
        validity: 'Geçerlilik',
        since: '{from} itibarıyla',
        between: '{from} – {to}',
        importedAt: 'Aktarıldı',
    },
    logo: {
        title: 'Logo',
        hint: 'PNG, JPEG, WebP veya SVG, en fazla 2 MB. Kenar çubuğunda ve giriş sayfasında görünür.',
        uploaded: 'Logo güncellendi',
        removed: 'Logo kaldırıldı',
        remove: 'Logoyu kaldır',
    },
    general: {
        organization: 'Organizasyon',
        organizationName: 'Organizasyon adı',
        organizationHint: 'Panelde, e-postalarda ve giriş sayfasında görünür.',
        regional: 'Dil ve bölge',
        regionalHint:
            'Yeni kullanıcılar için varsayılanlar; herkes kendi profilinden değiştirebilir.',
        defaultLocale: 'Varsayılan dil',
        defaultTimezone: 'Varsayılan saat dilimi',
        dateFormat: 'Tarih biçimi',
        timeFormat: 'Saat biçimi',
        timeFormats: { '24h': '24 saat (14:30)', '12h': '12 saat (2:30 ÖS)' },
        example: 'Örnek: {value}',
        saved: 'Ayarlar kaydedildi',
    },
    security: {
        twoFactor: 'İki adımlı doğrulama',
        twoFactorHint:
            'Burada seçilen roller paneli kullanmadan önce bir doğrulayıcı uygulama kurmak zorundadır. Varsayılan olarak hiçbir rol için zorunlu değildir.',
        requiredFor: 'Zorunlu olduğu roller',
        noneRequired: 'Hiçbir rol için zorunlu değil',
        sessions: 'Oturumlar',
        idleHours: 'Hareketsizlikte çıkış (saat)',
        lifetimeDays: 'En uzun oturum süresi (gün)',
        roles: {
            owner: 'Sahip',
            admin: 'Yönetici',
            manager: 'Müdür',
            agent: 'Temsilci',
        },
    },
    email: {
        title: 'Giden e-posta',
        description:
            'Davetler, şifre sıfırlama ve otomasyon e-postaları için kullanılır. Ayarlanmazsa davet bağlantıları kopyalamanız için panelde gösterilir.',
        managedByEnv:
            'E-posta sunucuda SMTP_* ortam değişkenleriyle yapılandırılmış. Buradan yönetmek için SMTP_HOST değişkenini kaldırın.',
        enabled: 'E-posta açık',
        disabled: 'E-posta kapalı',
        host: 'SMTP sunucusu',
        hostPlaceholder: 'smtp.ornek.com',
        port: 'Port',
        portHint: 'Güvenlik moduna göre varsayılan port için boş bırakın.',
        security: 'Güvenlik',
        tls: {
            starttls: 'STARTTLS (port 587)',
            tls: 'TLS (port 465)',
            none: 'Yok (yalnızca yerel test sunucuları)',
        },
        username: 'Kullanıcı adı',
        password: 'Şifre',
        passwordKeep: 'Kayıtlı — değiştirmek istemiyorsanız boş bırakın',
        from: 'Gönderen',
        fromPlaceholder: 'Acme <no-reply@acme.com>',
        test: 'Test e-postası gönder',
        testTo: 'Testin gideceği adres',
        testSent: 'Test e-postası {to} adresine gönderildi',
        turnOff: 'E-postayı kapat',
        saved: 'E-posta ayarları kaydedildi',
    },
};
export default settings;
