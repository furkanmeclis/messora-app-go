import type { Messages } from '../en';

const params: Messages['params'] = {
    fields: {
        name: 'Ad soyad',
        first_name: 'Ad',
        phone: 'Telefon',
        email: 'E-posta',
        var1: 'Değişken 1',
        var2: 'Değişken 2',
        var3: 'Değişken 3',
        var4: 'Değişken 4',
        var5: 'Değişken 5',
    },
    attribute: 'Özel alan: {key}',
    modes: { field: 'Kişi alanı', text: 'Sabit metin' },
    header: 'Başlık {{{name}}}',
    body: 'Gövde {{{name}}}',
    button: 'Buton {n}: adresin sonu',
    example: 'Örnek: {value}',
    chooseField: 'Alan seçin',
    searchField: 'Alan ara',
    noField: 'Böyle bir alan yok',
    textPlaceholder: 'Herkese gidecek metni yazın',
    fallback: 'Boşsa',
    fallbackPlaceholder: 'Kullanılacak değer (yoksa kişi atlanır)',
    fallbackHint: '{field} boş olan kişiler, bir değer vermezseniz atlanır.',
    noVariables: 'Bu şablonda değişken yok.',
    media: {
        title: 'Başlık {kind}',
        image: 'Görsel',
        video: 'Video',
        document: 'Belge',
        location: 'Konum',
        upload: 'Yükle',
        link: 'Bağlantı',
        linkPlaceholder: 'https://…',
        linkHint:
            'Herkese açık bir https adresi. Meta gönderirken buradan indirir.',
        uploadHint:
            "Gönderen numara üzerinden Meta'ya yüklenir; 30 gün kullanılabilir.",
        uploading: 'Yükleniyor…',
        uploaded: 'Yüklendi',
        remove: 'Kaldır',
        needSender: 'Önce gönderen numarayı seçin.',
        httpsOnly: 'https ile başlayan bir adres girin.',
        accept: {
            image: 'JPEG veya PNG, en fazla 5 MB',
            video: 'MP4, en fazla 16 MB',
            document: 'PDF ve diğer belgeler, en fazla 100 MB',
        },
    },
    issues: {
        title: 'Göndermeden önce',
        media: 'Başlık {kind} ekleyin.',
        location: 'Konum başlıklı şablonlar henüz gönderilemiyor.',
        value: '{slot} için bir değer gerekli.',
    },
    preview: {
        title: 'Önizleme',
        as: '{name} böyle görecek',
        sample: 'Örnek değerler',
    },
};

export default params;
