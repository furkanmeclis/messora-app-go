import type { Messages } from '../en';

const auth: Messages['auth'] = {
    email: 'E-posta',
    password: 'Şifre',
    newPassword: 'Yeni şifre',
    yourName: 'Adınız',
    passwordHint:
        'En az 12 karakter. Yaygın şifrelerden ve e-posta adresinizden kaçının.',
    login: {
        title: 'Giriş yap',
        description: 'Messora’ya tekrar hoş geldiniz.',
        submit: 'Giriş yap',
        forgot: 'Şifrenizi mı unuttunuz?',
        passwordChanged:
            'Şifreniz değiştirildi. Yeni şifrenizle giriş yapın.',
        expired: 'Giriş denemesinin süresi doldu. Lütfen yeniden giriş yapın.',
    },
    verify: {
        title: 'İki adımlı doğrulama',
        description: 'Doğrulama uygulamanızdaki 6 haneli kodu girin.',
        recoveryCode: 'Kurtarma kodu',
        authenticatorCode: 'Doğrulama kodu',
        submit: 'Doğrula',
        useApp: 'Bunun yerine doğrulama uygulamasını kullan',
        useRecovery: 'Bunun yerine kurtarma kodu kullan',
    },
    setup: {
        title: 'Messora kurulumu',
        description: 'Bu kurulum için sahip hesabını oluşturun.',
        organizationName: 'Kuruluş adı',
        organizationPlaceholder: 'Örnek A.Ş.',
        submit: 'Sahip hesabını oluştur',
        openLinkTitle: 'Kurulum bağlantısını açın',
        openLinkLogs:
            'Tek kullanımlık bağlantı, ilk açılışta API günlüklerine yazılır:',
        newLink: 'Yeni bir bağlantı için şu komutu çalıştırın:',
    },
    invite: {
        metaTitle: 'Daveti kabul et',
        title: 'Messora’ya katılın',
        invitedAs: '{role} rolüyle davet edildiniz.',
        accept: 'Davetinizi kabul edin.',
        incomplete: 'Bu davet bağlantısı eksik.',
        askAdmin: 'Bir yöneticiden yeni bir davet isteyin.',
        submit: 'Katıl',
    },
    forgot: {
        metaTitle: 'Şifre sıfırlama',
        title: 'Şifrenizi sıfırlayın',
        description:
            'Yeni bir şifre belirlemeniz için size e-postayla bir bağlantı göndereceğiz.',
        submit: 'Sıfırlama bağlantısı gönder',
        noEmail:
            'Bu kurulumda e-posta yapılandırılmamış. Erişiminizi sıfırlaması için bir sahibe ya da yöneticiye başvurun.',
        backToSignIn: 'Girişe dön',
        sent: 'Bu adrese ait bir hesap varsa şifre sıfırlama bağlantısı gönderdik. Gelen kutunuzu kontrol edin.',
    },
    reset: {
        title: 'Yeni şifre belirleyin',
        description: 'Tüm cihazlardaki oturumlarınız kapatılacak.',
        submit: 'Yeni şifreyi kaydet',
    },
};
export default auth;
