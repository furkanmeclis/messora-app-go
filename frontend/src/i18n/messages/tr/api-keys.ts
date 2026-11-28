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
    base: {
        title: 'API adresiniz',
        description:
            'Her çalışma alanının kendi API adresi vardır. Bu çalışma alanının anahtarları yalnızca burada çalışır; başka bir adreste 401 yanıtı alırlar.',
        copy: 'Taban adresi kopyala',
        copied: 'Taban adres kopyalandı',
        sample: 'Bir anahtarı deneyin',
    },
    never: 'Süresiz',
    revoke: 'İptal et',
    revoked: 'API anahtarı iptal edildi.',
    revokedMany: '{count} API anahtarı iptal edildi.',
    revokeManyTitle: '{count} API anahtarı iptal edilsin mi?',
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
