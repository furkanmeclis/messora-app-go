// Form definitions shared by the site (markup) and the server (validation).
// type: text | email | tel | url | textarea | select | checkbox

const L = (tr, en) => ({ tr, en });
const opt = (value, tr, en) => ({ value, label: L(tr, en) });

const name = { name: 'name', type: 'text', required: true, max: 120, autocomplete: 'name', label: L('Adınız soyadınız', 'Full name') };
const email = { name: 'email', type: 'email', required: true, max: 160, autocomplete: 'email', label: L('E-posta adresiniz', 'E-mail address') };
const workEmail = { ...email, label: L('İş e-postanız', 'Work e-mail') };
const phone = (required = false) => ({ name: 'phone', type: 'tel', required, max: 40, autocomplete: 'tel', label: L('Telefon', 'Phone') });
const company = (required = false) => ({ name: 'company', type: 'text', required, max: 160, autocomplete: 'organization', label: L('Şirket', 'Company') });
const message = (required = true) => ({ name: 'message', type: 'textarea', required, max: 4000, label: L('Mesajınız', 'Your message') });
const consent = { name: 'consent', type: 'checkbox', required: true, label: L('KVKK Aydınlatma Metni’ni okudum; bu talebe yanıt verilmesi için bilgilerimin işlenmesini kabul ediyorum.', 'I have read the Privacy Notice and agree that my details are processed to answer this request.') };
const teamSize = { name: 'team', type: 'select', required: true, label: L('Ekip büyüklüğü', 'Team size'), options: [
  opt('1-2', '1–2 kişi', '1–2 people'), opt('3-5', '3–5 kişi', '3–5 people'), opt('6-12', '6–12 kişi', '6–12 people'), opt('13+', '13 kişi ve üzeri', '13 or more') ] };

export const forms = {
  contact: {
    subject: L('İletişim formu', 'Contact form'),
    fields: [
      name, workEmail, company(), phone(),
      { name: 'topic', type: 'select', required: true, label: L('Konu', 'Topic'), options: [
        opt('sales', 'Satış ve paketler', 'Sales and plans'), opt('support', 'Teknik destek', 'Technical support'),
        opt('billing', 'Fatura ve ödeme', 'Billing and payment'), opt('partner', 'İş ortaklığı', 'Partnership'),
        opt('press', 'Basın', 'Press'), opt('other', 'Diğer', 'Other') ] },
      message(), consent,
    ],
  },
  demo: {
    subject: L('Demo talebi', 'Demo request'),
    fields: [
      name, workEmail, company(true), phone(true), teamSize,
      { name: 'interest', type: 'select', required: false, label: L('En çok ilgilendiğiniz', 'Most interested in'), options: [
        opt('inbox', 'Ekip gelen kutusu', 'Team inbox'), opt('campaigns', 'Kampanyalar ve toplu mesaj', 'Campaigns and broadcasts'),
        opt('ai', 'AI ajanları', 'AI agents'), opt('automations', 'Otomasyonlar', 'Automations'),
        opt('api', 'API ve entegrasyon', 'API and integrations'), opt('all', 'Hepsi', 'All of it') ] },
      { ...message(false), label: L('Kısaca ihtiyacınız (isteğe bağlı)', 'Your use case in short (optional)') },
      consent,
    ],
  },
  partner: {
    subject: L('İş ortaklığı başvurusu', 'Partner application'),
    fields: [
      name, workEmail, company(true), { name: 'website', type: 'url', required: false, max: 200, autocomplete: 'url', label: L('Web siteniz', 'Website') }, phone(),
      { name: 'ptype', type: 'select', required: true, label: L('İş ortaklığı türü', 'Partner type'), options: [
        opt('agency', 'Dijital ajans', 'Digital agency'), opt('software', 'Yazılım evi / entegratör', 'Software house / integrator'),
        opt('consultant', 'Danışman', 'Consultant'), opt('referral', 'Referans (yönlendirme)', 'Referral'), opt('other', 'Diğer', 'Other') ] },
      { name: 'clients', type: 'select', required: false, label: L('WhatsApp ile ilgilenen müşteri sayınız', 'Clients interested in WhatsApp'), options: [
        opt('1-5', '1–5', '1–5'), opt('6-20', '6–20', '6–20'), opt('21+', '21 ve üzeri', '21 or more'), opt('unknown', 'Henüz bilmiyorum', 'Not sure yet') ] },
      { ...message(true), label: L('Bize kendinizden ve müşterilerinizden bahsedin', 'Tell us about you and your clients') },
      consent,
    ],
  },
  lp: {
    subject: L('Reklam sayfası demo talebi', 'Landing page demo request'),
    fields: [name, workEmail, phone(true), company(true), consent],
  },
  kvkk: {
    subject: L('KVKK başvurusu / veri talebi', 'Data subject request'),
    kvkk: true,
    fields: [
      { name: 'rtype', type: 'select', required: true, label: L('Talebiniz', 'Your request'), options: [
        opt('info', 'Verilerimin işlenip işlenmediğini öğrenmek ve bilgi talep etmek', 'Find out whether my data is processed and get information'),
        opt('access', 'Verilerimin bir kopyasını almak', 'Get a copy of my data'),
        opt('rectify', 'Eksik veya yanlış verilerin düzeltilmesi', 'Correct incomplete or wrong data'),
        opt('delete', 'Verilerimin silinmesi veya yok edilmesi', 'Delete or destroy my data'),
        opt('whatsapp', 'WhatsApp / Meta üzerinden gelen verilerimin silinmesi', 'Delete my data received through WhatsApp / Meta'),
        opt('object', 'İşlemeye itiraz / açık rızamı geri almak', 'Object to processing / withdraw my consent'),
        opt('transfer', 'Verilerimin aktarıldığı üçüncü kişileri öğrenmek', 'Learn which third parties received my data'),
        opt('damage', 'Zararın giderilmesini talep etmek', 'Claim compensation for damage'),
        opt('other', 'Diğer', 'Other') ] },
      { name: 'relation', type: 'select', required: true, label: L('Messora ile ilişkiniz', 'Your relationship with Messora'), options: [
        opt('visitor', 'Web sitesi ziyaretçisi / form dolduran', 'Website visitor / form sender'),
        opt('user', 'Messora müşterisi veya kullanıcısı', 'Messora customer or user'),
        opt('enduser', 'Messora kullanan bir işletmeyle WhatsApp’ta yazıştım', 'I chatted on WhatsApp with a business that uses Messora'),
        opt('supplier', 'Tedarikçi / iş ortağı çalışanı', 'Supplier / partner employee'),
        opt('other', 'Diğer', 'Other') ] },
      name,
      { name: 'idno', type: 'text', required: true, max: 40, label: L('T.C. kimlik numarası (yabancılar için uyruk ve pasaport numarası)', 'National ID number (for foreigners: nationality and passport number)') },
      email,
      phone(false),
      { name: 'waphone', type: 'tel', required: false, max: 40, label: L('İlgili WhatsApp numarası (WhatsApp verisi için)', 'Related WhatsApp number (for WhatsApp data)') },
      { name: 'business', type: 'text', required: false, max: 160, label: L('Yazıştığınız işletme (varsa)', 'The business you chatted with (if any)') },
      { name: 'address', type: 'textarea', required: true, max: 600, autocomplete: 'street-address', label: L('Tebligata esas adresiniz', 'Postal address for notices') },
      { name: 'message', type: 'textarea', required: true, max: 6000, label: L('Talebinizin ayrıntıları', 'Details of your request') },
      { name: 'consent', type: 'checkbox', required: true, label: L('Verdiğim bilgilerin doğru olduğunu ve başvurumun yanıtlanması için işlenmesini kabul ediyorum.', 'I confirm the information is correct and agree that it is processed to answer my request.') },
    ],
  },
};
