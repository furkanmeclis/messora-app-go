// E-mail bodies for form submissions: the internal notification and the
// branded confirmation sent to the requester (TR/EN).
const esc = (s) => String(s ?? '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
const oneLine = (s) => String(s ?? '').replace(/[\r\n]+/g, ' ').slice(0, 80);

function valueOf(f, v, lang) {
  if (f.type === 'checkbox') return v ? (lang === 'tr' ? 'Evet' : 'Yes') : (lang === 'tr' ? 'Hayır' : 'No');
  if (f.type === 'select') return f.options.find((o) => o.value === v)?.label[lang] ?? v;
  return v;
}

export function internalMail({ type, spec, ref, data, meta }) {
  const rows = spec.fields.map((f) => [f.label.tr, valueOf(f, data[f.name], 'tr')]).filter(([, v]) => v !== '');
  const metaRows = [['Dil', meta.lang], ['Sayfa', meta.page], ...Object.entries(meta.attribution)];
  const who = oneLine(data.name) + (data.company ? ` (${oneLine(data.company)})` : '');
  const subject = `[Messora] ${spec.subject.tr}: ${who} · ${ref}`;
  const kvkk = spec.kvkk ? '<p style="margin:0 0 16px;padding:12px 14px;background:#FFF6DB;border-radius:10px;color:#5C4300"><b>KVKK başvurusu.</b> Yasal yanıt süresi başvurunun alındığı tarihten itibaren en geç 30 gündür. Kimlik doğrulaması yapmadan veri paylaşmayın.</p>' : '';
  const table = (rs) => `<table cellpadding="0" cellspacing="0" style="width:100%;border-collapse:collapse;font-size:14px">${rs.map(([k, v]) =>
    `<tr><td style="padding:8px 12px 8px 0;border-bottom:1px solid #E3E7F2;color:#4A5468;vertical-align:top;width:34%">${esc(k)}</td><td style="padding:8px 0;border-bottom:1px solid #E3E7F2;color:#0B1220;white-space:pre-wrap">${esc(v)}</td></tr>`).join('')}</table>`;
  const html = `<div style="font-family:Inter,Arial,sans-serif;max-width:640px;margin:0 auto;padding:24px;color:#0B1220">
<p style="margin:0 0 4px;font-size:12px;letter-spacing:.08em;text-transform:uppercase;color:#3B4CCA;font-weight:700">${esc(spec.subject.tr)}</p>
<h1 style="margin:0 0 16px;font-size:20px">${esc(ref)}</h1>${kvkk}
${table(rows)}
<p style="margin:20px 0 6px;font-size:12px;color:#4A5468">Kaynak</p>${table(metaRows)}
<p style="margin:20px 0 0;font-size:12px;color:#4A5468">Bu e-postayı yanıtlarsanız yanıtınız doğrudan başvuru sahibine gider.</p></div>`;
  const text = `${spec.subject.tr} — ${ref}\n\n${rows.map(([k, v]) => `${k}: ${v}`).join('\n')}\n\n${metaRows.map(([k, v]) => `${k}: ${v}`).join('\n')}\n`;
  return { subject, html, text };
}

const copy = {
  tr: {
    hi: (n) => `Merhaba ${n},`,
    got: {
      contact: 'Mesajınız bize ulaştı. İş günlerinde, genellikle aynı gün içinde size dönüyoruz.',
      demo: 'Demo talebiniz bize ulaştı. İş günlerinde size e-postayla dönüp uygun bir görüşme saati öneriyoruz.',
      partner: 'İş ortaklığı başvurunuz bize ulaştı. İnceleyip iş günlerinde size dönüyoruz.',
      lp: 'Demo talebiniz bize ulaştı. İş günlerinde sizi arıyor ya da yazıyoruz.',
      kvkk: 'Kişisel verilerinizle ilgili başvurunuz bize ulaştı. 6698 sayılı KVKK’nın 13. maddesi uyarınca başvurunuzu talebin niteliğine göre en kısa sürede ve en geç 30 gün içinde ücretsiz olarak sonuçlandırıyoruz. Kimliğinizi doğrulamak için sizinle bu e-posta adresinden iletişime geçebiliriz.',
    },
    ref: 'Referans numaranız',
    keep: 'Bu numarayı yazışmalarınızda belirtmeniz işlemi hızlandırır.',
    summary: 'Gönderdiğiniz bilgiler',
    sign: 'Messora ekibi',
    foot: 'Bu e-posta, messora.tech üzerindeki formu doldurduğunuz için gönderildi. Formu siz doldurmadıysanız bu e-postayı dikkate almayın.',
    subject: (s, r) => `${s} alındı · ${r}`,
  },
  en: {
    hi: (n) => `Hello ${n},`,
    got: {
      contact: 'We have received your message. We reply on business days, usually the same day.',
      demo: 'We have received your demo request. We will e-mail you on business days to suggest a time for the call.',
      partner: 'We have received your partner application. We will review it and get back to you on business days.',
      lp: 'We have received your demo request. We will call or write to you on business days.',
      kvkk: 'We have received your request about your personal data. Under Article 13 of the Turkish Personal Data Protection Law No. 6698 (KVKK) we conclude it free of charge as soon as possible and within 30 days at the latest. We may contact you at this address to verify your identity.',
    },
    ref: 'Your reference number',
    keep: 'Quoting this number when you write to us speeds things up.',
    summary: 'What you sent',
    sign: 'The Messora team',
    foot: 'You are receiving this e-mail because the form on messora.tech was filled in with this address. If it wasn’t you, please ignore it.',
    subject: (s, r) => `${s} received · ${r}`,
  },
};

const LOGO = `<table cellpadding="0" cellspacing="0"><tr><td style="width:30px;height:30px;background:#3B4CCA;border-radius:8px 8px 8px 2px;text-align:center;color:#fff;font:800 17px/30px Arial,sans-serif">m</td><td style="padding-left:9px;font:800 20px/1 Arial,sans-serif;letter-spacing:-.5px;color:#F4F6FB">messora</td></tr></table>`;

export function confirmationMail({ type, spec, ref, data, lang }) {
  const c = copy[lang];
  const skip = new Set(['consent', 'idno', 'address']);
  const rows = spec.fields.filter((f) => !skip.has(f.name)).map((f) => [f.label[lang], valueOf(f, data[f.name], lang)]).filter(([, v]) => v !== '');
  const name = oneLine(data.name).split(' ')[0] || '';
  const subject = c.subject(spec.subject[lang], ref);
  const html = `<!doctype html><html lang="${lang}"><body style="margin:0;background:#F4F6FB">
<div style="max-width:600px;margin:0 auto;padding:24px 16px;font-family:Inter,Arial,sans-serif;color:#0B1220">
<div style="background:#0B1220;border-radius:18px 18px 0 0;padding:22px 26px">${LOGO}</div>
<div style="background:#fff;border-radius:0 0 18px 18px;padding:28px 26px">
<p style="margin:0 0 14px;font-size:16px">${esc(c.hi(name))}</p>
<p style="margin:0 0 20px;font-size:15px;line-height:1.6;color:#2A3347">${esc(c.got[type])}</p>
<div style="background:#F4F6FB;border-radius:14px 14px 14px 4px;padding:16px 18px;margin:0 0 20px">
<p style="margin:0;font-size:12px;letter-spacing:.06em;text-transform:uppercase;color:#4A5468">${esc(c.ref)}</p>
<p style="margin:4px 0 0;font:800 22px/1.2 Arial,sans-serif;color:#3B4CCA;letter-spacing:.5px">${esc(ref)}</p>
<p style="margin:6px 0 0;font-size:13px;color:#4A5468">${esc(c.keep)}</p></div>
<p style="margin:0 0 8px;font-size:13px;font-weight:700;color:#4A5468">${esc(c.summary)}</p>
<table cellpadding="0" cellspacing="0" style="width:100%;border-collapse:collapse;font-size:14px">${rows.map(([k, v]) =>
    `<tr><td style="padding:7px 10px 7px 0;border-bottom:1px solid #E3E7F2;color:#4A5468;vertical-align:top;width:38%">${esc(k)}</td><td style="padding:7px 0;border-bottom:1px solid #E3E7F2;white-space:pre-wrap">${esc(v)}</td></tr>`).join('')}</table>
<p style="margin:24px 0 0;font-size:15px">${esc(c.sign)}</p>
<p style="margin:2px 0 0;font-size:13px;color:#4A5468">Konuşmak, satmanın en kısa yolu.</p></div>
<p style="margin:16px 8px 0;font-size:12px;line-height:1.5;color:#4A5468">${esc(c.foot)} <a href="https://messora.tech" style="color:#3B4CCA">messora.tech</a></p>
</div></body></html>`;
  const text = `${c.hi(name)}\n\n${c.got[type]}\n\n${c.ref}: ${ref}\n${c.keep}\n\n${c.summary}:\n${rows.map(([k, v]) => `${k}: ${v}`).join('\n')}\n\n${c.sign}\nhttps://messora.tech\n`;
  return { subject, html, text };
}
