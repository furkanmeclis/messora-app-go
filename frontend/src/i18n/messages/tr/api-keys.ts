import type { Messages } from '../en';

const apiKeys: Messages['apiKeys'] = {
    title: 'API anahtarları',
    description:
        'Entegrasyonlar için kimlik bilgileri. `Authorization: Bearer msk_live_…` başlığıyla kullanın.',
    empty: 'Henüz API anahtarı yok',
    emptyHint:
        "Başka bir sistemi Messora'ya bağlamak için bir anahtar oluşturun.",
    columns: {
        name: 'Ad',
        scopes: 'Yetki kapsamları',
        lastUsed: 'Son kullanım',
        expires: 'Son geçerlilik',
    },
    never: 'Süresiz',
    revoke: 'İptal et',
    revoked: 'API anahtarı iptal edildi.',
    create: {
        button: 'Anahtar oluştur',
        title: 'API anahtarı oluşturun',
        description: 'Yalnızca entegrasyonun ihtiyaç duyduğu yetkileri verin.',
        copyNow:
            'Anahtarı şimdi kopyalayın. Güvenliğiniz için bir daha gösterilmeyecek.',
        copyKey: 'Anahtarı kopyala',
        keyCopied: 'Anahtar kopyalandı',
        done: 'Tamam',
        name: 'Ad',
        namePlaceholder: 'CRM senkronizasyonu',
        expiresAfter: 'Geçerlilik süresi (gün)',
        expiresHint: 'Süresiz bir anahtar için boş bırakın.',
        scopes: 'Yetki kapsamları',
    },
};
export default apiKeys;
