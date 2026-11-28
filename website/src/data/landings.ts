// Google Ads landing pages (Turkish, noindex). Headline matches the search term.
import type { LandingData } from '../views/Landing.astro';

const common = {
  faq: [
    { q: 'Meta’nın mesaj ücretleri pakete dahil mi?', a: 'Hayır. Meta, iletilen şablon mesajlar için kategori ve ülkeye göre ayrıca ücret alır. Messora bu ücreti göndermeden önce gösterir.' },
    { q: 'Ne kadar sürede başlarım?', a: 'Messora hesabınız hemen açılır. Meta’nın işletme doğrulaması ve şablon onayı Meta’ya bağlıdır ve birkaç gün sürebilir; sihirbaz her adımı gösterir.' },
    { q: 'Taahhüt var mı?', a: 'Aylık paketlerde yok; istediğiniz zaman iptal edersiniz. Yıllık ödemede 12 ay 10 ay fiyatına.' },
    { q: 'Verilerimiz nerede?', a: 'AWS Frankfurt’ta, Avrupa Birliği’nde. Her firmanın verisi ayrıdır, sırlar şifrelenir, her gece şifreli yedek alınır.' },
  ],
};

export const landings: Record<string, LandingData> = {
  'whatsapp-business-api': {
    ...common,
    path: '/lp/whatsapp-business-api',
    title: 'WhatsApp Business API — resmi Cloud API ile başlayın',
    description: 'Resmi WhatsApp Business API’yi ekip gelen kutusu, kampanya ve AI ajanlarıyla kullanın. 14 gün ücretsiz, kart gerekmez. Meta ücreti göndermeden görünür.',
    eyebrow: 'Resmi WhatsApp Business API',
    h1: 'WhatsApp Business API’ye resmi yoldan, bugün başlayın.',
    lead: 'Messora, Meta’nın resmi Cloud API’si üzerinde çalışır. Ekibiniz tek gelen kutusunda yanıtlar, izinli kampanyalar gönderir, AI sık soruları karşılar.',
    points: ['QR kod değil, Meta’nın resmi API’si', 'Kurulum sihirbazı: Meta bağlantısı adım adım', 'Veriler AB’de (AWS Frankfurt)'],
    mock: 'inbox',
    why: [
      { h: 'Numaranız güvende', p: 'Gayriresmî araçlar numaranın kapanmasına yol açabilir. Resmi API’de gönderimler Meta onaylı şablonlarla ve kalite kurallarıyla yapılır.' },
      { h: 'Ekip için tasarlandı', p: 'Konuşmayı atayın, not bırakın, devredin. Her konuşmanın bir sahibi olur.' },
      { h: 'Maliyet ortada', p: 'Meta’nın ücreti kampanya başlamadan önce ülke ve kategoriye göre hesaplanır.' },
    ],
  },
  'whatsapp-toplu-mesaj': {
    ...common,
    path: '/lp/whatsapp-toplu-mesaj',
    title: 'WhatsApp toplu mesaj — izinli ve resmi API ile',
    description: 'WhatsApp toplu mesajı resmi API ile, yalnızca izin verenlere gönderin. Maliyet ve bitiş saati göndermeden önce görünür. 14 gün ücretsiz deneyin.',
    eyebrow: 'WhatsApp toplu mesaj',
    h1: 'WhatsApp toplu mesaj: kime, ne kadara, ne zaman — göndermeden bilin.',
    lead: 'Kitleyi segment, grup ya da etiketle seçin. Mesaj almak istemeyenler otomatik çıkarılır; Meta ücreti ve tahmini bitiş saati ekranda.',
    points: ['İzin ve ret kategoriye göre kayıtlı', 'DUR yazan kişi hemen listeden çıkar', 'Kalite uyarısında kampanya kendiliğinden durur'],
    mock: 'campaigns',
    why: [
      { h: 'Yasal çerçeveye saygılı', p: 'Ticari ileti için gerekli izinleri kaydedin; izin vermeyen kişiye şablon gönderilmez.' },
      { h: 'Adil gönderim', p: 'Aynı numarada süren kampanyalar hızı adil paylaşır; acil bildirimleriniz sıkışmaz.' },
      { h: 'Sonuçları görün', p: 'Teslim, okunma, yanıt ve olumlu/olumsuz dönüşler kampanya raporunda; dışa aktarım XLSX ve PDF.' },
    ],
  },
  'whatsapp-chatbot': {
    ...common,
    path: '/lp/whatsapp-chatbot',
    title: 'WhatsApp chatbot — yapay zekâ ajanı, insana devirle',
    description: 'WhatsApp chatbot yerine ekibin yanında çalışan AI ajanı: sık soruları yanıtlar, bilgileri toplar, emin olmadığında temsilciye devreder. 14 gün ücretsiz.',
    eyebrow: 'WhatsApp yapay zekâ ajanı',
    h1: 'WhatsApp chatbot’tan fazlası: emin olmadığında ekibe devreden AI.',
    lead: 'OpenAI, Anthropic veya Gemini ile çalışan ajanlar müşterilerinizi 7/24 karşılar, adı, e-postayı ve şehri kişi kartına yazar, gerektiğinde konuşmayı temsilciye geçirir.',
    points: ['“Temsilci” yazılınca konuşma ekibe geçer', 'Her numara için ayrı AI profili', 'Konuşmalar model eğitiminde kullanılmaz'],
    mock: 'ai',
    why: [
      { h: 'Senaryo yazmadan', p: 'Karar ağacı çizmek zorunda değilsiniz; ajan, AI profilinde tanımladığınız bilgilerle ve kurallarla yanıt verir.' },
      { h: 'Ölçülü kullanım', p: 'AI kullanımı aylık AI kredisiyle ölçülür; hangi konuşmada ne kadar harcandığı görünür.' },
      { h: 'Otomasyonla birlikte', p: '“AI ile otomatik cevap” ve “Müşteri bilgisi toplama” hazır şablonlarıyla dakikalar içinde açılır.' },
    ],
  },
  'whatsapp-crm': {
    ...common,
    path: '/lp/whatsapp-crm',
    title: 'WhatsApp CRM — kişiler, etiketler ve ekip gelen kutusu',
    description: 'WhatsApp CRM: kişileri etiketleyin, segmentleyin, konuşmaları ekibe atayın. API ve imzalı webhook’larla kendi sisteminize bağlayın. 14 gün ücretsiz.',
    eyebrow: 'WhatsApp CRM',
    h1: 'WhatsApp CRM: her müşteri, geçmişiyle ve izniyle tek yerde.',
    lead: 'Kişileri içe aktarın, etiketleyin, dinamik segmentler oluşturun. Konuşmalar atanır, notlanır; kampanya yanıtları doğru kişiye düşer.',
    points: ['CSV/XLSX içe ve dışa aktarım', 'Etiket, grup, özel alan ve dinamik segment', 'REST API ve imzalı webhook’lar'],
    mock: 'automations',
    why: [
      { h: 'Konuşma ve kişi aynı yerde', p: 'Müşteri panelinde izin durumu, etiketler ve notlar konuşmanın yanında durur.' },
      { h: 'Otomatik düzen', p: 'Olumlu dönüş yapanı satış temsilcisine atayın, sessiz kalanı etiketleyin; hazır otomasyonlarla.' },
      { h: 'Kendi sisteminize bağlanır', p: 'Sipariş, randevu veya CRM kayıtlarınızı API ile eşleyin; olaylar imzalı webhook’la size gelir.' },
    ],
  },
};
