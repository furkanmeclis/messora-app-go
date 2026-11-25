import type { Messages } from '../en';

const offers: Messages['offers'] = {
    title: 'Teklifler',
    description:
        'Kampanyalarınızın tanıttığı süreli teklifler. Süreli teklif şablonları WhatsApp’ta bitişe kadar geri sayar.',
    nav: {
        label: 'Şablonlar ve teklifler',
        templates: 'Şablonlar',
        offers: 'Teklifler',
    },
    new: 'Yeni teklif',
    edit: 'Teklifi düzenle',
    create: 'Teklifi oluştur',
    dialogHint:
        'Teklife bağlı kampanyalar bitişini, kuponunu ve görselini kullanır; teklif bitince durur.',
    status: 'Durum',
    statuses: {
        draft: 'Taslak',
        scheduled: 'Planlandı',
        active: 'Aktif',
        expired: 'Bitti',
    },
    fields: {
        title: 'Başlık',
        discount: 'İndirim',
        discountPlaceholder: '%25 indirim',
        coupon: 'Kupon kodu',
        starts: 'Başlangıç',
        ends: 'Bitiş',
        product: 'Ürün veya hizmet',
        description: 'Açıklama',
        image: 'Görsel (https)',
        cta: 'Teklif sayfası',
        template: 'Şablon',
    },
    noTemplate: 'Şablon yok',
    ltoTag: 'geri sayım',
    publish: 'Yayınla (başlangıç ile bitiş arasında aktif)',
    preview: 'Önizleme',
    ltoNote:
        'Süreli teklif şablonu: WhatsApp bitişe kadar geri sayımı ve kuponu gösterir. WhatsApp Web ve masaüstü, teklifin orada desteklenmediğini gösterir.',
    fallbackNote:
        'WhatsApp geri sayımını yalnızca süreli teklif şablonları gösterir. Normal bir pazarlama şablonunda teklifin görseli boş görsel başlığını doldurur; {{offer_title}}, {{offer_discount}}, {{offer_coupon}}, {{offer_ends}} ve {{offer_url}} yer tutucuları gerisini taşır.',
    empty: 'Henüz teklif yok',
    emptyHint:
        'Bir teklif oluşturup kampanyaya bağlayın; teklif bitince kampanya durur.',
    editLabel: '{title} düzenle',
    endLabel: '{title} teklifini şimdi bitir',
    deleteLabel: '{title} sil',
    deleteTitle: '{title} silinsin mi?',
    deleteBody:
        'Onu kullanan kampanyalar kendi değerlerini korur; hiçbir şey yeniden gönderilmez.',
    created: 'Teklif oluşturuldu.',
    saved: 'Teklif kaydedildi.',
    ended: 'Teklif bitirildi; onu kullanan kampanyalar duraklatıldı.',
    deleted: 'Teklif silindi.',
    campaign: {
        label: 'Teklif',
        none: 'Teklif yok',
        hint: 'Bağlı teklif geri sayım bitişini, kuponu ve görseli doldurur; bitince kampanyayı duraklatır.',
        ended: 'Bu teklif bitti; kampanya bu teklifle başlatılamaz.',
    },
};
export default offers;
