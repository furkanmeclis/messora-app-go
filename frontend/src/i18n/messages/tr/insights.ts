import type { Messages } from '../en';

const insights: Messages['insights'] = {
    tabs: { insights: 'Analiz', definition: 'Tanım' },
    kpi: {
        contacts: 'Kişi',
        newThisMonth: '30 günde +{count}',
        reachable: 'Ulaşılabilir',
        optedOut: '{count} abonelikten çıktı',
        active: 'Aktif (30 gün)',
        active7: 'Bu hafta {count}',
        delivery: 'Teslim oranı',
        readRate: 'Okunma oranı',
        replyRate: 'Yanıt oranı',
        ofSent: '{count} gönderimden',
        ofDelivered: '{count} teslimden',
        ofMessaged: '{total} kişiden {count}',
    },
    activity: {
        title: 'Etkinlik',
        description: 'Son 30 günde bu kişilerle mesajlaşma.',
        sent: 'Gönderilen',
        delivered: 'Teslim edilen',
        read: 'Okunan',
        inbound: 'Gelen',
    },
    funnel: {
        title: 'Mesaj hunisi',
        description: 'Son 90 gün, bu kişilere giden tüm mesajlar.',
        sent: 'Gönderilen',
        delivered: 'Teslim edilen',
        read: 'Okunan',
        replied: 'Yanıt veren (kişi)',
        failed: '{count} başarısız',
        fromCampaigns: '{count} kampanyadan',
    },
    campaigns: {
        title: 'Bu kişilere ulaşan kampanyalar',
        description:
            'Bu kişilerden en az birine mesaj gönderen her kampanya; ölçümler yalnızca bu kişiler üzerinden.',
        empty: 'Henüz bu kişilere ulaşan bir kampanya yok.',
        name: 'Kampanya',
        reach: 'Ulaşılan',
        reachOf: '{total} kişiden {reached}',
        delivered: 'Teslim',
        read: 'Okunma',
        replied: 'Yanıt',
        failed: 'Başarısız',
        optOuts: 'Sonra çıkan',
        started: 'Başlangıç',
        targeted: 'Doğrudan',
        targetedHint: 'Bu kitleye gönderildi',
        indirect: 'Kesişim',
        indirectHint:
            'Bu kişilerin bir kısmını içeren başka bir kitleye gönderildi',
        compare: 'Kampanyalara göre okunma oranı',
    },
    engaged: {
        title: 'En etkileşimli kişiler',
        description: 'Son 30 günde en çok yazanlar.',
        empty: 'Son 30 günde kimse yazmadı.',
        messages: '{count} mesaj',
    },
    group: {
        notFound: 'Grup bulunamadı',
        back: 'Gruplara dön',
        summary: '{count} üye',
    },
    tag: {
        notFound: 'Etiket bulunamadı',
        back: 'Etiketlere dön',
        summary: 'Bu etiketi taşıyan {count} kişi',
    },
    viewContacts: 'Kişileri gör',
    noAccess: 'Analiz için kampanyaları görme yetkisi gerekir.',
};

export default insights;
