import type { Messages } from '../en';

const account: Messages['account'] = {
    profile: {
        title: 'Profil',
        description: 'Ekibinize nasıl göründüğünüz.',
        personalDetails: 'Kişisel bilgiler',
        signedInAs: '{email} olarak giriş yaptınız ({role}).',
        name: 'Ad soyad',
        timezone: 'Saat dilimi',
        language: 'Dil',
        saved: 'Profil kaydedildi.',
    },
    security: {
        title: 'Güvenlik',
        description: 'Şifre, iki adımlı doğrulama ve açık oturumlar.',
        mfaRequiredTitle: 'İki adımlı doğrulama zorunlu',
        mfaRequiredBody:
            'Rolünüz bir doğrulayıcı uygulama kullanmanızı gerektiriyor. Panele devam etmek için aşağıdan kurulumu tamamlayın.',
    },
    password: {
        title: 'Şifre',
        description:
            'Şifrenizi değiştirdiğinizde diğer tüm cihazlardan çıkış yapılır.',
        current: 'Mevcut şifre',
        new: 'Yeni şifre',
        newHint: 'En az 12 karakter.',
        submit: 'Şifreyi değiştir',
        changed: 'Şifre değiştirildi. Diğer oturumlar kapatıldı.',
    },
    sessions: {
        title: 'Açık oturumlar',
        description: 'Hesabınızda şu anda oturumu açık olan cihazlar.',
        thisDevice: 'Bu cihaz',
        unknownAddress: 'Bilinmeyen adres',
        lastActive: '{ip} · son etkinlik {time}',
        signOut: 'Çıkış yap',
        signOutOthers: 'Diğer tüm oturumları kapat',
        unknownDevice: 'Bilinmeyen cihaz',
        browser: 'Tarayıcı',
        browserOn: '{os} üzerinde {browser}',
        ended: 'Oturum kapatıldı.',
        othersEnded: 'Diğer tüm oturumlar kapatıldı.',
    },
    twoFactor: {
        title: 'İki adımlı doğrulama',
        description:
            'Girişte, 1Password, Google Authenticator veya Aegis gibi bir doğrulayıcı uygulamadan alınan kod da istenir.',
        on: 'Açık',
        off: 'Kapalı',
        qrAlt: 'Doğrulayıcı uygulamanız için QR kod',
        scan: 'QR kodu okutun ya da bu anahtarı elle girin:',
        code: 'Uygulamadaki kod',
        turnOn: 'Aç',
        turnOff: 'Kapat',
        setUpNow: 'Şimdi kur',
        newCodes: 'Yeni kurtarma kodları',
        turnedOff: 'İki adımlı doğrulama kapatıldı.',
        recoveryIntro:
            'Bu kurtarma kodlarını güvenli bir yerde saklayın. Telefonunuzu kaybederseniz her biri bir kez kullanılabilir.',
        recoveryWarning: 'Bu kodlar bir daha gösterilmeyecek.',
        recoveryCopied: 'Kurtarma kodları kopyalandı',
        savedThem: 'Kaydettim',
    },
    reauth: {
        title: 'Kimliğinizi doğrulayın',
        description:
            'Bu işlem için yeniden doğrulama gerekiyor. Doğrulama 10 dakika geçerli kalır.',
        password: 'Şifre',
        code: '…ya da doğrulayıcı uygulamadaki kod',
        confirm: 'Onayla',
    },
};
export default account;
