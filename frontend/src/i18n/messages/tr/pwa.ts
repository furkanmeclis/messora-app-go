import type { Messages } from '../en';

const pwa: Messages['pwa'] = {
    tab: 'Uygulama (PWA)',
    title: 'Yüklenebilir uygulama',
    description:
        'Ekibiniz paneli bilgisayarlara ve telefonlara uygulama olarak yükleyebilsin: kendi penceresi ve simgesiyle, seçtiğiniz sayfada açılır.',
    enabled: 'Uygulama olarak yüklemeyi sun',
    enabledHint:
        'Tarayıcılar yükleme seçeneği gösterir; panelde de “Uygulamayı yükle” düğmesi çıkar.',
    name: 'Uygulama adı',
    namePlaceholder: 'Boş bırakılırsa organizasyon adı',
    shortName: 'Kısa ad',
    shortNameHint:
        'Ana ekranda simgenin altında görünür (en fazla 20 karakter).',
    appDescription: 'Açıklama',
    themeColor: 'Başlık çubuğu rengi',
    backgroundColor: 'Açılış ekranı rengi',
    display: 'Pencere',
    displays: {
        standalone: 'Uygulama penceresi',
        fullscreen: 'Tam ekran',
        'minimal-ui': 'Sade tarayıcı çubuğu',
    },
    startUrl: 'Açılış sayfası',
    icon: 'Simge',
    icons: {
        brand: 'Tema renginde Messora simgesi',
        logo: 'Organizasyon logosu',
    },
    noLogo: "Kullanmak için Görünüm'den bir logo yükleyin.",
    preview: 'Önizleme',
    install: 'Uygulamayı yükle',
    installed: 'Yüklü',
    installHint:
        'Yüklemenin ne zaman mümkün olduğuna tarayıcınız karar verir (HTTPS, daha önce yüklenmemiş olmalı).',
    saved: 'Uygulama ayarları kaydedildi',
};
export default pwa;
