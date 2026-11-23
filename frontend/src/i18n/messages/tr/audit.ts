import type { Messages } from '../en';

const audit: Messages['audit'] = {
    title: 'Denetim kaydı',
    description:
        'Güvenlikle ilgili tüm işlemler, en yeniden eskiye. Kayıtlar değiştirilemez ve silinemez.',
    filteredBy: 'Filtre:',
    clearFilter: 'Filtreyi temizle',
    columns: {
        when: 'Zaman',
        who: 'Kim',
        action: 'İşlem',
        details: 'Ayrıntılar',
    },
    empty: 'Kayıt yok.',
    system: 'Sistem',
    unknown: 'Bilinmiyor',
    loadMore: 'Daha fazla yükle',
};
export default audit;
