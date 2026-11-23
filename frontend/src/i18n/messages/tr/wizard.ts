import type { Messages } from '../en';

const wizard: Messages['wizard'] = {
    title: '{organization} kurulumu',
    welcome: "Messora'ya hoş geldiniz",
    welcomeBody:
        "Birkaç adımda çalışma alanınız hazır. Her adım atlanabilir ve sonradan Ayarlar'dan değiştirilebilir.",
    start: 'Başla',
    stepOf: 'Adım {current} / {total}',
    steps: {
        organization: 'Organizasyon',
        language: 'Dil',
        region: 'Tarih ve saat',
        theme: 'Tema',
        email: 'E-posta',
        whatsapp: 'WhatsApp',
        security: 'Güvenlik',
        team: 'Ekip',
    },
    organization: {
        title: 'Organizasyonunuz',
        body: 'Ad ve logo panelde ve giriş sayfasında görünür.',
    },
    language: {
        title: 'Dil',
        body: 'Sizin için panel dili ve ekibiniz için varsayılan.',
    },
    region: {
        title: 'Tarih ve saat',
        body: 'Tarih ve saatlerin panelde nasıl görüneceği.',
    },
    theme: {
        title: 'Bir görünüm seçin',
        body: "Bir tema seçin. Her rengi sonradan Ayarlar → Görünüm'den ince ayarlayabilirsiniz.",
    },
    email: {
        title: 'Giden e-posta',
        body: 'Davet ve şifre sıfırlama e-postaları göndermek için bir SMTP sunucusu bağlayın.',
    },
    whatsapp: {
        title: "WhatsApp'ı bağlayın",
        body: "Meta'daki WhatsApp Business hesabınızı ekleyin. WABA ID ve kalıcı erişim token'ı gerekir.",
        connected: '{name} bağlandı.',
        openChannels: 'Hesap bağla',
        later: "Hesapları istediğiniz zaman Kanallar'dan bağlayabilirsiniz.",
    },
    security: {
        title: 'İki adımlı doğrulama',
        body: 'Hangi rollerin girişini bir doğrulayıcı uygulamayla koruması gerektiğini seçin.',
        recommendation: 'Sahip ve yöneticiler için önerilir.',
    },
    team: {
        title: 'Ekibinizi davet edin',
        body: "Kişileri şimdi ya da sonra Kullanıcılar'dan davet edin.",
        email: 'E-posta adresi',
        role: 'Rol',
        addAnother: 'Bir kişi daha ekle',
        invite: 'Davetleri gönder',
        invited: '{count} davet gönderildi',
        linkHint:
            'E-posta kapalı; her davet bağlantısını kopyalayıp kendiniz gönderin:',
    },
    done: {
        title: 'Hazırsınız',
        body: 'Çalışma alanınız hazır. Sıradaki adımlar:',
        next: {
            import: 'Kişilerinizi içe aktarın',
            template: 'Mesaj şablonlarınızı eşitleyin',
            automation: 'Bir karşılama otomasyonu oluşturun',
        },
        open: 'Paneli aç',
    },
};
export default wizard;
