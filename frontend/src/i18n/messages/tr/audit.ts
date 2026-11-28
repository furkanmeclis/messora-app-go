import type { Messages } from '../en';

const audit: Messages['audit'] = {
    title: 'Denetim kaydı',
    description:
        'Güvenlikle ilgili tüm işlemler, en yeniden eskiye. Kayıtlar değiştirilemez ve silinemez.',
    filteredBy: 'Filtre:',
    clearFilter: 'Filtreyi temizle',
    filterBy: 'Yalnızca {action} kayıtlarını göster',
    columns: {
        when: 'Zaman',
        who: 'Kim',
        action: 'İşlem',
        details: 'Ayrıntılar',
    },
    support: {
        title: 'Messora destek erişimi',
        description:
            'Messora destek ekibinin bu çalışma alanını açtığı zamanlar; her zaman sınırlı süreyle ve bir nedenle.',
        when: 'Zaman',
        who: 'Kim',
        reason: 'Neden',
        access: 'Erişim',
        active: 'Şu an',
    },
    empty: 'Kayıt yok.',
    system: 'Sistem',
    unknown: 'Bilinmiyor',
};
export default audit;
