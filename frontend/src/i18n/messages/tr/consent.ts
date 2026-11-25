import type { Messages } from '../en';

const consent: Messages['consent'] = {
    statuses: {
        opted_in: 'İzin verdi',
        unknown: 'Bilinmiyor',
        opt_out_requested: 'Çıkmak istedi',
        opted_out: 'Abonelikten çıktı',
        resubscribed: 'Yeniden abone oldu',
    },
    categories: {
        marketing: 'Pazarlama',
        service: 'Hizmet',
        all: 'İkisi de',
    },
    categoryHints: {
        marketing: 'Kampanyalar, fırsatlar ve diğer pazarlama şablonları.',
        service:
            'Sipariş bildirimi, doğrulama kodu gibi hizmet ve kimlik doğrulama şablonları.',
        all: 'Pazarlama ve hizmet birlikte.',
    },
    flags: {
        metaStopped: 'WhatsApp’ta durdurdu',
        metaStoppedHint:
            'Müşteri WhatsApp’ta pazarlama mesajlarınız için “Durdur”u seçti. WhatsApp’ta “Devam et”i seçene kadar pazarlama mesajı gönderilmez.',
        undeliverable: 'Ulaşılamıyor',
        undeliverableHint:
            'Meta bu numaraya mesaj iletemedi. Müşteri yeniden yazana kadar hiçbir mesaj gönderilmez.',
        undeliverableCode: 'Meta hatası {code}',
    },
    reasons: {
        opted_out: 'Abonelikten çıktı',
        meta_marketing_stopped: 'WhatsApp’ta pazarlamayı durdurdu',
        undeliverable: 'Numaraya ulaşılamıyor',
        opt_out_requested: 'Mesajları durdurmak istedi',
        consent_unknown: 'Kayıtlı izni yok',
        replied_after_opt_out: 'Çıktıktan sonra yeniden yazdı',
    },
    reasonHelp: {
        opted_out: 'Bu tür mesajları almak istemediğini bildirdi.',
        meta_marketing_stopped: 'WhatsApp’ta pazarlama mesajlarınızı kapattı.',
        undeliverable:
            'Meta numaraya mesaj iletemedi; müşteri yeniden yazınca yine ulaşılabilir olur.',
        opt_out_requested:
            'Mesajları durdurmak istedi. Bir temsilci çıkışı onaylayana ya da talebi kaldırana kadar pazarlama mesajı gönderilmez.',
        consent_unknown: 'Pazarlama mesajlarına izin verdiğine dair kayıt yok.',
        replied_after_opt_out:
            'Abonelikten çıktı ama yeniden yazdı; o görüşmede yanıt verilebilir.',
    },
    events: {
        opt_in: 'İzin verdi',
        opt_out: 'Abonelikten çıktı',
        opt_out_requested: 'Mesajları durdurmak istedi',
        resubscribed: 'Yeniden abone oldu',
        unknown: 'Bilinmiyor olarak ayarlandı',
        meta_marketing_stopped: 'WhatsApp’ta pazarlamayı durdurdu',
        meta_marketing_resumed: 'WhatsApp’ta pazarlamayı yeniden açtı',
        undeliverable: 'Meta ulaşılamıyor olarak işaretledi',
        deliverable: 'Yeniden ulaşılabilir',
    },
    sources: {
        manual: 'elle',
        bulk: 'toplu işlem',
        api: 'API',
        import: 'içe aktarma',
        keyword: 'mesajdaki anahtar kelime',
        meta_user_preferences: 'WhatsApp ayarı',
        meta_error: 'Meta hatası',
        automation: 'otomasyon',
        inbound: 'müşteri mesajı',
    },
    card: {
        title: 'Abonelik ve izin',
        description:
            'Pazarlama şablonları için pazarlama izni, hizmet ve kimlik doğrulama şablonları için hizmet izni gerekir.',
        change: 'İzni değiştir',
        confirmOptOut: 'Çıkışı onayla',
        clearRequest: 'Abone kalsın',
        requestPending:
            'Mesajları durdurmak istedi. Çıkışı onaylayana ya da talebi kaldırana kadar pazarlama mesajı gönderilmez.',
        updated: '{time} tarihinde güncellendi',
        reason: 'Neden: {reason}',
    },
    dialog: {
        title: 'İzni değiştir',
        description:
            'Her değişiklik adınız ve nedeniyle birlikte izin geçmişinde saklanır.',
        category: 'Kategori',
        status: 'Yeni durum',
        reason: 'Neden',
        reasonPlaceholder: 'ör. Telefonda kampanya almak istemediğini söyledi',
        reasonRequired: 'Kişinin neden abonelikten çıktığını yazın.',
        hardNote:
            'WhatsApp’ta pazarlamayı durduran müşterilere ve Meta’nın ulaşamadığı numaralara, burada ne seçilirse seçilsin mesaj gönderilmez.',
        save: 'Kaydet',
        saved: 'İzin güncellendi.',
    },
    history: {
        title: 'İzin geçmişi',
        empty: 'Henüz izin değişikliği yok.',
        loadFailed: 'İzin geçmişi yüklenemedi.',
        by: '{name} tarafından',
        automatic: 'otomatik olarak',
        via: 'kaynak: {source}',
        both: 'ikisi de',
    },
    filters: {
        marketing: 'Pazarlama izni',
        service: 'Hizmet izni',
        reachability: 'Ulaşılabilirlik',
    },
    preflight: {
        title: 'Bu mesajı kimler alacak',
        checking: 'Kitlenin izinleri kontrol ediliyor…',
        failed: 'İzinler kontrol edilemedi: {message}',
        audience: 'Seçtiğiniz kitlede {n} kişi var',
        refused: ', bunlardan {n} kişi mesaj almak istemediğini belirtmiş.',
        blockedOnly: ', bunlardan {n} kişiye bu mesaj gönderilemez.',
        allClear: ', hepsi bu mesajı alabilir.',
        eligible: '{n} kişi alacak',
        eligibleWarned: '{n} kişi alacak, {warned} kişi için uyarıyla',
        blocked: 'Gönderilmeyecek ({n})',
        warned: 'Uyarıyla gönderilecek ({n})',
        hard: 'Her zaman engelli',
        category: {
            marketing: 'Bu bir pazarlama şablonu: pazarlama izni gerekir.',
            utility:
                'Bu bir hizmet şablonu: pazarlama izni değil, hizmet izni gerekir.',
            authentication:
                'Bu bir kimlik doğrulama şablonu: pazarlama izni değil, hizmet izni gerekir.',
        },
        hardNote:
            'Abonelikten çıkanlara, WhatsApp’ta pazarlamanızı durduranlara ve ulaşılamayan numaralara asla mesaj gönderilmez: hiçbir ayar, API çağrısı ya da otomasyon bunu değiştiremez.',
        unknownPolicy: {
            allow: 'Şirketiniz kayıtlı izni olmayan kişilere pazarlama mesajını uyarıyla gönderiyor.',
            block: 'Şirketiniz kayıtlı izni olmayan kişilere pazarlama mesajı göndermiyor.',
        },
        changePolicy: 'İzin ayarlarından değiştir',
        noneEligible: 'Bu kitlede bu şablonu alabilecek kimse yok.',
        confirmEligible: '“{template}” şablonu {n} kişiye hemen gönderilecek. ',
    },
    settings: {
        tab: 'Abonelik ve izin',
        title: 'Abonelik ve izin',
        description:
            'Kimin hangi mesajları alabileceği ve müşterilerin size yazarak mesajları nasıl durdurup yeniden başlatabileceği.',
        hardTitle: 'Her zaman engelli',
        hardBody:
            'Bir kategoriden çıkan kişiler, WhatsApp’ta pazarlamanızı durduran müşteriler ve Meta’nın ulaşamadığı numaralar bu mesajları hiçbir zaman almaz. Ayarlar, API ve otomasyonlar bunu değiştiremez.',
        unknownTitle: 'Kayıtlı izni olmayanlara pazarlama',
        unknownDescription:
            'Elle eklenen, içe aktarılan ya da size ilk kez yazan kişilerin çoğunun kayıtlı pazarlama izni yoktur.',
        unknown: {
            allow: 'Uyarıyla gönder',
            allowHint:
                'Başlangıç için önerilir. Kampanya özetinde kaç kişinin kayıtlı izni olmadığı gösterilir.',
            block: 'Gönderme',
            blockHint:
                'Pazarlama mesajlarını yalnızca izin veren ya da yeniden abone olan kişiler alır.',
        },
        requestNote:
            'Bir çıkış talebi (“lütfen artık yazmayın” gibi bir ifade), bir temsilci çıkışı onaylayana ya da talebi kaldırana kadar o kişiye pazarlama mesajı gönderilmesini durdurur.',
        keywordsTitle: 'Anahtar kelimeler',
        keywordsDescription:
            'Müşterilerden gelen mesajlar bu listelerle karşılaştırılır. Büyük/küçük harf ve Türkçe karakter farkı gözetilmez; her listede en fazla 50 kayıt olabilir.',
        stop: 'Durdurma kelimeleri',
        stopHint:
            'Mesaj tam olarak bunlardan biriyse kişi pazarlama mesajlarından hemen çıkarılır.',
        request: 'Çıkış talebi ifadeleri',
        requestHint:
            'Bunlardan birini içeren mesaj, bir temsilcinin onaylaması için işaretlenir; bu sırada pazarlama mesajı gönderilmez.',
        resume: 'Yeniden abonelik kelimeleri',
        resumeHint:
            'Mesaj tam olarak bunlardan biriyse kişi pazarlama mesajlarına yeniden abone olur.',
        confirmTitle: 'Onay yanıtları',
        confirm: 'Onay yanıtı gönder',
        confirmHint:
            'Durdurma ya da yeniden abonelik kelimesinden sonra kişiye kendi dilinde bu yanıt gönderilir.',
        stopMessage: 'Durdurma kelimesinden sonra',
        resumeMessage: 'Yeniden abonelik kelimesinden sonra',
        tr: 'Türkçe',
        en: 'İngilizce',
        saved: 'İzin ayarları kaydedildi.',
    },
    notification: {
        optOutRequested: 'Bir müşteri mesajları durdurmak istedi.',
        open: 'Görüşmeyi aç',
    },
    failureClasses: {
        rejected_by_meta: 'Meta reddetti',
        temporary: 'Geçici hata',
        permanent: 'Kalıcı hata',
        user_stopped_marketing: 'Müşteri pazarlamayı durdurdu',
        undeliverable: 'Ulaşılamıyor',
        ecosystem_limit: 'Meta erteledi',
        blocked_by_consent: 'İzin nedeniyle engellendi',
        cancelled: 'İptal edildi',
    },
};

export default consent;
