import type { Messages } from '../en';

const license: Messages['license'] = {
    tab: 'Lisans',
    title: 'Lisans',
    description:
        'Messora lisansınız: kime ait olduğu, ne zamana kadar geçerli olduğu ve hangi güncellemeleri kapsadığı.',
    status: {
        trial: 'Deneme',
        active: 'Etkin',
        expiring: 'Süresi doluyor',
        grace: 'Ek süre',
        restricted: 'Kısıtlı',
    },
    reason: {
        trial_ended: 'Deneme süresi bitti.',
        expired: 'Lisansın süresi doldu.',
        revoked: 'Lisans iptal edildi.',
        superseded: 'Lisansın yerine yeni bir anahtar verildi.',
        check_in_overdue: 'Messora uzun süredir lisans sunucusuna ulaşamadı.',
    },
    until: {
        trial: 'Deneme {date} tarihinde bitiyor',
        active: '{date} tarihine kadar geçerli',
        expiring: '{date} tarihinde sona eriyor',
        grace: 'Gönderim {date} tarihinde duracak',
        restricted: '{date} tarihinden beri kısıtlı',
    },
    perpetual: 'Süresiz',
    customer: 'Lisans sahibi',
    plan: 'Plan',
    issuedAt: 'Veriliş',
    expiresAt: 'Bitiş',
    updatesUntil: 'Güncellemeler şu tarihe kadar dahil',
    allUpdates: 'Gelecekteki tüm güncellemeler',
    checkIn: 'Doğrulama',
    checkInOnline: 'Çevrim içi (birkaç saatte bir kontrol eder)',
    checkInOffline: 'Çevrim dışı (bağlantı gerekmez)',
    lastCheckIn: 'Son kontrol: {date}',
    neverCheckedIn: 'Henüz kontrol edilmedi',
    limits: 'Sınırlar',
    unlimited: 'Sınırsız',
    users: '{count} kullanıcı',
    numbers: '{count} numara',
    instance: 'Kurulum kimliği',
    instanceHint: 'Lisansınızla ilgili destek isterken bunu iletin.',
    copy: 'Kopyala',
    copied: 'Kopyalandı',
    keyTitle: 'Lisans anahtarı',
    keyDescription:
        'Size verilen anahtarı yapıştırın. Hemen doğrulanır; eski anahtar artık kullanılmaz.',
    keyPlaceholder: 'msl1.…',
    keySave: 'Anahtarı etkinleştir',
    keySaved: 'Lisans etkinleştirildi.',
    ownerOnly: 'Lisans anahtarını yalnızca sahip girebilir.',
    noLicense: 'Henüz lisans anahtarı girilmedi.',
    update: {
        title: 'Güncellemeler',
        latest: 'En yeni sürüm: {version}',
        current: 'Çalışan: {version}',
        included: 'Lisansınıza dahil.',
        notIncluded:
            'Güncelleme süreniz bittikten sonra yayınlandı: kurmak için yenileyin.',
        none: 'Henüz güncelleme bilgisi yok.',
    },
    banner: {
        trial: 'Deneme: {days} gün kaldı. Gönderime devam etmek için lisans anahtarı girin.',
        expiring: 'Lisansınız {date} tarihinde sona eriyor.',
        grace: '{reason} Gönderim {date} tarihinde duracak.',
        restricted:
            '{reason} Gönderim, kampanyalar ve otomasyonlar durduruldu; verileriniz güvende.',
        action: 'Lisans ayarları',
    },
};

export default license;
