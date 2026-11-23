import type { Messages } from '../en';

const auth: Messages['auth'] = {
    email: 'E-posta',
    password: 'Parola',
    newPassword: 'Yeni parola',
    yourName: 'Adınız',
    passwordHint:
        'En az 12 karakter. Yaygın parolalardan ve e-posta adresinizden kaçının.',
    login: {
        title: 'Giriş yap',
        description: 'Messora’ya tekrar hoş geldiniz.',
        submit: 'Giriş yap',
        forgot: 'Parolanızı mı unuttunuz?',
        passwordChanged:
            'Parolanız değiştirildi. Yeni parolanızla giriş yapın.',
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
        metaTitle: 'Parola sıfırlama',
        title: 'Parolanızı sıfırlayın',
        description:
            'Yeni bir parola belirlemeniz için size e-postayla bir bağlantı göndereceğiz.',
        submit: 'Sıfırlama bağlantısı gönder',
        noEmail:
            'Bu kurulumda e-posta yapılandırılmamış. Erişiminizi sıfırlaması için bir sahibe ya da yöneticiye başvurun.',
        backToSignIn: 'Girişe dön',
        sent: 'Bu adrese ait bir hesap varsa parola sıfırlama bağlantısı gönderdik. Gelen kutunuzu kontrol edin.',
    },
    reset: {
        title: 'Yeni parola belirleyin',
        description: 'Tüm cihazlardaki oturumlarınız kapatılacak.',
        submit: 'Yeni parolayı kaydet',
    },
};
export default auth;
