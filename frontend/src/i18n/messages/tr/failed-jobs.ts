import type { Messages } from '../en';

const failedJobs: Messages['failedJobs'] = {
    title: 'Başarısız işler',
    description:
        'Tekrar deneme hakkı bitene kadar başarısız olan arka plan işleri. Sorunu giderdikten sonra yeniden deneyin ya da silin.',
    showing: '{total} işin en yeni {shown} tanesi gösteriliyor.',
    allQueues: 'Tüm kuyruklar',
    queues: {
        critical: 'Kritik',
        campaigns: 'Kampanyalar',
        ai: 'Yapay zekâ',
        default: 'Varsayılan',
        bulk: 'Toplu',
    },
    columns: {
        type: 'İş',
        queue: 'Kuyruk',
        error: 'Son hata',
        failedAt: 'Başarısız olduğu an',
        attempts: 'Denemeler',
        refs: 'Referanslar',
    },
    attempts: '{max} tekrardan {retried} tanesi',
    retry: 'Yeniden dene',
    delete: 'Sil',
    retryTitle: 'Bu iş yeniden denensin mi?',
    retryBody:
        'İş şimdi bir kez daha çalışır. Yine başarısız olursa doğrudan bu listeye döner.',
    deleteTitle: 'Bu iş silinsin mi?',
    deleteBody:
        'İş atılır ve bir daha çalışmaz. Üzerinde çalıştığı kayıtlar mevcut durumlarında kalır.',
    retried: 'İş yeniden kuyruğa alındı',
    deleted: 'İş silindi',
    retriedMany: '{count} iş yeniden kuyruğa alındı',
    deletedMany: '{count} iş silindi',
    retryManyTitle: '{count} iş yeniden denensin mi?',
    deleteManyTitle: '{count} iş silinsin mi?',
    empty: 'Başarısız iş yok',
    emptyHint:
        'Tüm arka plan işleri ya tamamlandı ya da hâlâ yeniden deneniyor.',
};
export default failedJobs;
