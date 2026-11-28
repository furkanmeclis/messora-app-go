import type { Solution } from './types';

export const solutions: Solution[] = [
  {
    key: "ecommerce",
    slug: "e-ticaret",
    nav: "E-ticaret",
    icon: "cart",
    seoTitle: "E-ticaret için WhatsApp Business API",
    seoDescription: "E-ticarette sipariş ve kargo bildirimlerini WhatsApp'tan gönderin, sık soruları yapay zekaya bırakın, kampanyaları yalnızca izinli müşterilere ulaştırın.",
    eyebrow: "E-ticaret",
    h1: "E-ticaret için WhatsApp: sipariş sorularını otomatiğe, satışı ekibinize bırakın",
    lead: "Sipariş onayı, kargo bilgisi ve iade soruları WhatsApp'ta cevaplansın; kampanyalar yalnızca izin veren müşterilere gitsin. Messora mağaza sisteminize API ve webhook'larla bağlanır.",
    summary: "Sipariş ve kargo bildirimleri, yapay zekayla sık soru cevapları ve izinli müşterilere kampanyalar.",
    pains: [
      {
        title: "\"Kargom nerede?\" soruları ekibi boğuyor",
        body: "Gün boyu aynı soruya elle cevap veriliyor. Bu arada ürün hakkında soru soran, satın almaya hazır müşteri sırada bekliyor ve kayboluyor.",
      },
      {
        title: "Kampanya kime gidiyor, kaça mal oluyor belli değil",
        body: "Listeler karışık, kimin mesaj almak istediği bilinmiyor. Gönderim maliyeti ancak fatura gelince görülüyor; istemeyen müşteriye giden her mesaj numaranın kalitesini riske atıyor.",
      },
      {
        title: "Tek telefon, birkaç kişi",
        body: "Mağaza numarası bir telefonda duruyor. Kimin hangi müşteriye cevap verdiği belli değil; müşteri aynı soruyu iki kez soruyor, bazı mesajlar hiç cevaplanmıyor.",
      },
    ],
    flows: [
      {
        title: "Kargo bildirimi ve takip soruları",
        trigger: "Sipariş kargoya verildiğinde",
        steps: [
          "Mağaza sisteminiz kargo bilgisini Messora API'sine gönderir",
          "Bilgilendirme (utility) kategorisindeki kargo şablonu müşteriye takip numarasıyla gider",
          "Müşteri yanıt verirse \"AI ile otomatik cevap\" akışı teslimat ve iade kurallarınıza göre cevaplar",
          "İade, hasar ya da şikâyet gibi konularda yapay zeka konuşmayı özetiyle ekibe devreder",
        ],
        outcome: "Rutin kargo soruları ekibin önüne gelmeden cevaplanır; ekip insan gerektiren konuşmalarla ilgilenir.",
        features: ["api", "templates", "ai", "inbox"],
      },
      {
        title: "Sepet hatırlatması, yalnızca izinli müşterilere",
        trigger: "Müşteri sepetini tamamlamadan ayrıldığında",
        steps: [
          "Mağaza sisteminiz müşteriye API ile \"sepet-hatirlatma\" etiketi ekler",
          "Etiketle başlayan otomasyon bir süre bekler, ardından pazarlama şablonunu gönderir",
          "Pazarlama izni olmayan müşteriye mesaj gitmez; adım \"atlandı\" olarak kaydedilir",
          "Müşteri \"DUR\" yazarsa izni hemen kapanır, \"Abonelikten çıkma talebi\" akışı teşekkür eder",
        ],
        outcome: "Hatırlatma yalnızca mesaj almak isteyen müşterilere gider; numaranızın kalitesi korunur.",
        features: ["api", "automations", "templates"],
      },
      {
        title: "Yeni koleksiyon kampanyası",
        trigger: "Yeni ürünler yayına girdiğinde",
        steps: [
          "Son alışverişine ya da ilgi alanına göre bir segment oluşturun",
          "Ürünleri carousel pazarlama şablonuyla kart kart gösterin",
          "Ön analizde engellenen kişileri, Meta maliyetini ve bitiş zamanını kontrol edin",
          "\"Olumlu müşteri dönüşü\" akışı ilgilenen müşteriyi satış ekibinden birine atar",
        ],
        outcome: "Kampanyanın maliyetini baştan bilir, ilgilenen müşteriyi hemen bir temsilciyle buluşturursunuz.",
        features: ["campaigns", "templates", "automations", "analytics"],
      },
    ],
    templates: [
      {
        name: "kargo_bildirimi",
        category: "utility",
        body: "Merhaba {{1}}, {{2}} numaralı siparişiniz kargoya verildi. Kargo takip numaranız: {{3}}. Teslimatla ilgili sorularınızı bu sohbetten iletebilirsiniz.",
        buttons: ["Kargomu takip et"],
      },
      {
        name: "siparis_onayi",
        category: "utility",
        body: "Teşekkürler {{1}}, {{2}} numaralı siparişinizi aldık. Toplam tutar: {{3}}. Siparişiniz kargoya verildiğinde size buradan haber vereceğiz.",
      },
      {
        name: "sepet_hatirlatma",
        category: "marketing",
        body: "Merhaba {{1}}, sepetinize eklediğiniz ürünler sizi bekliyor. Siparişinizi tamamlamak için aşağıdaki bağlantıyı kullanabilirsiniz. Bu tür mesajları almak istemiyorsanız DUR yazmanız yeterli.",
        buttons: ["Sepete dön", "Mesaj istemiyorum"],
      },
    ],
    compliance: "Kampanya ve sepet hatırlatması gibi pazarlama mesajları 6563 sayılı Kanun kapsamında ticari elektronik iletidir: müşterinin önceden onayı gerekir ve onayların İleti Yönetim Sistemi'ne (İYS) kaydı sizin sorumluluğunuzdadır. Sepet hatırlatmasını yalnızca pazarlama izni olan müşterilere gönderin. Messora her müşterinin pazarlama ve hizmet izinlerini ayrı tutar, her değişikliğin kaynağını ve zamanını kaydeder; pazarlama iznini kapatan kişiye kampanya ya da otomasyon yoluyla pazarlama mesajı gitmez. Sipariş ve kargo bildirimleri bilgilendirme (utility) şablonudur. Meta pazarlama ve bilgilendirme mesajlarını farklı ücretlendirir; kategoriyi içeriğe göre doğru seçmek hem kurallara uyum hem maliyet için önemlidir.",
    faq: [
      {
        q: "Mağaza altyapımla hazır bir entegrasyon var mı?",
        a: "Hazır bir mağaza eklentisi yok. Mağaza sisteminiz Messora'ya REST API ve webhook'larla bağlanır: sipariş ve kargo olaylarını API'ye gönderen küçük bir entegrasyonla bildirimler, etiketler ve otomasyonlar çalışır. API çağrıları Büyüme planından itibaren açıktır.",
      },
      {
        q: "Ürün kataloğumu WhatsApp'ta gösterebilir miyim?",
        a: "Meta'nın katalog butonları şu an desteklenmiyor, çünkü Messora ürün kataloğu yönetmiyor. Ürünleri carousel şablonuyla 10 karta kadar, her kartta görsel ve bağlantı butonuyla gösterebilirsiniz.",
      },
      {
        q: "Bir sepet hatırlatma mesajı kaça mal olur?",
        a: "Sepet hatırlatması pazarlama kategorisindedir. 1 Nisan 2026 tarifesinde Türkiye için teslim edilen pazarlama mesajı yaklaşık 0,0109 USD'dir; güncel fiyatı Meta'nın resmi sayfasından kontrol edin. Kampanyalarda Messora tutarı göndermeden önce hesaplar.",
      },
      {
        q: "Kampanyadan sonra gelen soruları kim cevaplar?",
        a: "İsterseniz yapay zeka ajanı ilk cevabı verir ve gerektiğinde ekibe devreder. \"Kampanya sonrası takip\" akışı kampanyaya yanıt veren müşteriyi etiketler, not düşer ve bir temsilciye yönlendirir.",
      },
    ],
    features: ["campaigns", "automations", "ai", "api"],
  },
  {
    key: "retail",
    slug: "perakende",
    nav: "Perakende",
    icon: "store",
    seoTitle: "Perakende ve Mağazalar için WhatsApp",
    seoDescription: "Mağazanız için WhatsApp: ürün hazır bildirimleri, mağaza sorularına hızlı cevap, kuponlu süreli teklifler ve şehir bazlı kampanyalar, hepsi izin kontrolüyle.",
    eyebrow: "Perakende",
    h1: "Perakende için WhatsApp: mağaza müşterinizle kasadan sonra da konuşun",
    lead: "Ayırttığı ürün geldiğinde müşteriye haber verin, çalışma saatlerini soranı bekletmeyin, indirim dönemini kuponlu bir teklifle duyurun. Hepsi resmi WhatsApp Business API üzerinden ve izin kontrolüyle.",
    summary: "Ürün hazır bildirimleri, mağaza sorularına hızlı cevap ve izinli müşterilere kuponlu kampanyalar.",
    pains: [
      {
        title: "Mağaza soruları telefonu meşgul ediyor",
        body: "\"Bu ürün var mı?\", \"Kaçta kapanıyorsunuz?\" soruları gün boyu gelir. Kasadaki ekip hem müşteriye hem telefona yetişmeye çalışır.",
      },
      {
        title: "Müşterinin numarası var, izni belli değil",
        body: "Kasada toplanan numaralar bir listede birikir ama kimin kampanya mesajı almayı kabul ettiği kayıtlı değildir. İzinsiz gönderim hem yasal hem kalite riski taşır.",
      },
      {
        title: "Kampanya duyurusunun sonucu görülmüyor",
        body: "İndirim duyurusu gönderilir, ama kaç kişinin okuduğu, kimin ilgilendiği ve kaça mal olduğu bilinmez. Bir sonraki kampanya yine tahminle planlanır.",
      },
    ],
    flows: [
      {
        title: "Ayırtılan ürün hazır",
        trigger: "Müşterinin ayırttığı ürün mağazaya ulaştığında",
        steps: [
          "Stok ya da satış sisteminiz bilgiyi Messora API'sine gönderir",
          "Bilgilendirme (utility) kategorisindeki \"ürün hazır\" şablonu mağaza adı ve son alma tarihiyle gider",
          "Müşteri yanıt verirse konuşma o mağazanın temsilcisine atanır",
        ],
        outcome: "Müşteri ürünün geldiğini zamanında öğrenir; ayırtılan ürünler rafta unutulmaz.",
        features: ["api", "templates", "inbox"],
      },
      {
        title: "Mağaza bilgisi soruları",
        trigger: "Müşteri adres, çalışma saati ya da iade koşullarını sorduğunda",
        steps: [
          "\"AI ile otomatik cevap\" akışı talimatlarınızdaki mağaza bilgileriyle cevap verir",
          "Stok gibi yapay zekanın bilemeyeceği konularda konuşma ekibe devredilir",
          "\"Temsilciye devir\" akışı \"yetkili\" ya da \"canlı destek\" yazanı doğrudan bir temsilciye aktarır",
        ],
        outcome: "Sık sorulan sorular beklemeden cevaplanır, ekip yalnızca kendisine gereken konuşmalarla ilgilenir.",
        features: ["ai", "automations", "inbox"],
      },
      {
        title: "Şehre göre indirim duyurusu",
        trigger: "Yeni indirim dönemi başladığında",
        steps: [
          "Müşterinin şehri ya da mağazasına göre bir segment oluşturun",
          "Kupon kodlu süreli teklif şablonu hazırlayın; WhatsApp geri sayımı gösterir",
          "Ön kontrol yalnızca pazarlama izni olan müşterileri sayar, engellenenleri nedeniyle gösterir",
          "Teklif bittiğinde kampanya kendiliğinden durur; sonuçları şehir kırılımıyla inceleyin",
        ],
        outcome: "Duyuru yalnızca izin veren müşterilere gider; hangi şehrin ilgilendiğini rakamlarla görürsünüz.",
        features: ["campaigns", "templates", "analytics"],
      },
    ],
    templates: [
      {
        name: "urun_hazir",
        category: "utility",
        body: "Merhaba {{1}}, ayırttığınız {{2}} {{3}} mağazamızda sizi bekliyor. Ürün {{4}} tarihine kadar sizin için ayrıldı.",
        buttons: ["Yol tarifi al"],
      },
      {
        name: "sezon_indirimi",
        category: "marketing",
        body: "Merhaba {{1}}, {{2}} mağazamızda sezon indirimi başladı. Aşağıdaki kupon koduyla kasada ek indirimden yararlanabilirsiniz. Mesaj almak istemiyorsanız DUR yazabilirsiniz.",
        buttons: ["Kodu kopyala", "Kampanyayı gör"],
      },
    ],
    compliance: "Kasada ya da sadakat kartı başvurusunda alınan telefon numarası, tek başına pazarlama mesajı için onay sayılmaz. İndirim ve kampanya duyuruları ticari elektronik iletidir: müşterinin açık onayını alın, onayın nerede ve ne zaman alındığını kaydedin ve İYS'ye bildirin. Messora pazarlama ve hizmet izinlerini ayrı tutar ve her değişikliğin kaynağını saklar; izni olmayan kişiye pazarlama şablonu göndermez. Ürün hazır ya da sipariş durumu gibi bilgilendirmeler bilgilendirme (utility) şablonudur. Meta kategorileri farklı ücretlendirir; bilgilendirme şablonunun içine tanıtım eklerseniz Meta şablonu pazarlama olarak sınıflandırabilir.",
    faq: [
      {
        q: "Her mağazamız için ayrı numara kullanabilir miyiz?",
        a: "Pro planda 3 WhatsApp numarası bağlarsınız ve hepsinin konuşmaları aynı gelen kutusunda toplanır. Daha fazla numara için Kurumsal plan ihtiyacınıza göre belirlenir.",
      },
      {
        q: "Kasada topladığımız numaralara kampanya gönderebilir miyiz?",
        a: "Yalnızca müşteri pazarlama mesajı almayı açıkça kabul ettiyse. Numaraları içe aktardığınızda pazarlama izni \"bilinmiyor\" olarak başlar; onayı olan müşterileri işaretlemeniz ve İYS kaydını yapmanız gerekir.",
      },
      {
        q: "Mağaza ekipleri yalnızca kendi müşterileriyle mi konuşur?",
        a: "Konuşmaları ilgili mağazanın temsilcisine atayabilirsiniz. Temsilci rolü varsayılan olarak yalnızca kendisine atanan ve henüz atanmamış konuşmaları görür; yöneticiler hepsini görür.",
      },
      {
        q: "Kuponlu teklifin süresi bitince ne olur?",
        a: "Teklifin bitiş zamanı geldiğinde ona bağlı kampanya kendiliğinden duraklar, kalan müşteriler bekler. Süresi geçmiş bir teklifle kampanya başlatılamaz.",
      },
    ],
    features: ["campaigns", "templates", "inbox", "ai"],
  },
  {
    key: "health",
    slug: "saglik-ve-klinikler",
    nav: "Sağlık ve klinikler",
    icon: "health",
    seoTitle: "Klinikler için WhatsApp Randevu Hatırlatma",
    seoDescription: "Klinik ve muayenehaneler için WhatsApp: randevu hatırlatma ve teyit, mesai dışı sorulara yapay zeka cevabı ve sağlık verisini korumaya yönelik net kurallar.",
    eyebrow: "Sağlık ve klinikler",
    h1: "Klinikler için WhatsApp: randevuyu hatırlatın, sağlık verisini kanala taşımayın",
    lead: "Randevu hatırlatması ve teyidi WhatsApp'tan yapılsın, hasta \"değiştirmek istiyorum\" dediğinde sekreterliğe düşsün. Mesajlar randevu lojistiğiyle sınırlı kalsın; teşhis ve tahlil sonucu bu kanala girmesin.",
    summary: "Randevu hatırlatma ve teyit, mesai dışı sorulara ilk cevap ve sağlık verisine dikkat eden kurallar.",
    pains: [
      {
        title: "Randevu teyidi telefonda saatler alıyor",
        body: "Ertesi günün hastaları tek tek aranıyor, ulaşılamayanlar için yeniden deneniyor. Gelmeyen hasta boş kalan bir randevu saati demek.",
      },
      {
        title: "Mesai dışı sorular sabaha birikiyor",
        body: "Adres, otopark, randevu nasıl alınır gibi sorular akşam ve hafta sonu gelir. Sabah ekip önce bu birikmiş mesajları cevaplamakla uğraşır.",
      },
      {
        title: "Hasta bilgisi kişisel telefonlarda",
        body: "Çalışanlar hastalarla kendi WhatsApp hesaplarından yazışıyor. Hangi bilginin kimde olduğu, çalışan ayrılınca ne olacağı belli değil.",
      },
    ],
    flows: [
      {
        title: "Randevu hatırlatma ve teyit",
        trigger: "Randevudan bir gün önce",
        steps: [
          "Randevu sisteminiz hastanın adını, tarihini, saatini ve şubeyi Messora API'sine gönderir",
          "Bilgilendirme (utility) kategorisindeki hatırlatma şablonu \"Geliyorum\" ve \"Değiştirmek istiyorum\" butonlarıyla gider",
          "\"Değiştirmek istiyorum\" mesajıyla başlayan otomasyon konuşmayı sekreterliğe devreder ve not düşer",
          "Hatırlatmanın teslim ve okunma durumu webhook'la randevu sisteminize döner",
        ],
        outcome: "Sekreterlik yalnızca değişiklik isteyen hastalarla ilgilenir; telefonla tek tek teyit ihtiyacı azalır.",
        features: ["api", "templates", "automations", "inbox"],
      },
      {
        title: "Mesai dışı ilk cevap",
        trigger: "Klinik kapalıyken hasta yazdığında",
        steps: [
          "Yapay zeka ajanı adres, çalışma saatleri ve randevu süreci gibi lojistik soruları talimatlarınıza göre cevaplar",
          "Tıbbi bir soru geldiğinde kritik konu olarak algılar ve konuşmayı ekibe devreder",
          "Randevu isteyen hastanın adını ve tercih ettiği günü sorar; sağlık bilgisi istemez",
          "Sabah ekip, \"İnsan bekleyen\" listesinde her konuşmanın özetini görür",
        ],
        outcome: "Hasta mesajının görüldüğünü bilir; ekip sabah özetlerle ve öncelik sırasıyla başlar.",
        features: ["ai", "inbox"],
      },
      {
        title: "İlk kez yazan hasta",
        trigger: "Daha önce hiç yazmamış bir numaradan mesaj geldiğinde",
        steps: [
          "\"Yeni müşteri karşılama\" akışı kısa bir karşılama mesajı gönderir ve \"yeni-musteri\" etiketi ekler",
          "Ajan, profilde tanımlı alanları (ad, tercih edilen şube) birer birer sorar",
          "Bilgiler tamamlanınca konuşma randevu ekibine devredilir",
        ],
        outcome: "Yeni hastanın temel iletişim bilgileri, sağlık verisine dokunmadan toplanır.",
        features: ["automations", "ai", "inbox"],
      },
    ],
    templates: [
      {
        name: "randevu_hatirlatma",
        category: "utility",
        body: "Merhaba {{1}}, {{2}} tarihinde saat {{3}} için {{4}} şubemizde randevunuz bulunuyor. Katılım durumunuzu aşağıdan bildirebilirsiniz.",
        buttons: ["Geliyorum", "Değiştirmek istiyorum"],
      },
      {
        name: "randevu_olusturuldu",
        category: "utility",
        body: "Sayın {{1}}, randevunuz oluşturuldu: {{2}}, saat {{3}}. Adres: {{4}}. Randevunuzu değiştirmek için bu mesaja yanıt verebilirsiniz.",
      },
      {
        name: "giris_kodu",
        category: "authentication",
        body: "{{1}} doğrulama kodunuzdur. Güvenliğiniz için bu kodu kimseyle paylaşmayın.",
        buttons: ["Kodu kopyala"],
      },
    ],
    compliance: "Sağlık verisi, KVKK'nın 6. maddesine göre özel nitelikli kişisel veridir ve daha sıkı kurallara tabidir. WhatsApp mesajlarını randevu lojistiğiyle (tarih, saat, şube, adres) sınırlı tutun; teşhis, tahlil sonucu, ilaç ya da bölüm bilgisini şablonlara ve mesajlara koymayın. Randevu hatırlatmaları için hastadan açık rıza alın ve rızanın kaydını tutun. Messora'nın yapay zeka profillerinde sağlık bilgisi, kimlik numarası gibi alanlar tanımlanamaz ve ajana bunları istememesi, kaydetmemesi söylenir. Hasta kendiliğinden sağlık bilgisi yazarsa mesaj konuşma geçmişinde saklanır; bu nedenle hastalarınızı bu kanalda sağlık bilgisi paylaşmamaları konusunda bilgilendirin. Randevu bildirimleri bilgilendirme (utility), giriş kodları doğrulama (authentication) şablonudur; Meta kategorileri farklı ücretlendirir.",
    faq: [
      {
        q: "Tahlil sonuçlarını WhatsApp'tan gönderebilir miyiz?",
        a: "Önermiyoruz. Tahlil sonucu sağlık verisidir. WhatsApp mesajında yalnızca sonucun hazır olduğunu ve nereden görülebileceğini bildirin; sonucun kendisini kendi güvenli hasta portalınızda paylaşın.",
      },
      {
        q: "Hasta yapay zekaya tıbbi bir soru sorarsa ne olur?",
        a: "Profil talimatlarında ajanın tıbbi tavsiye vermemesini ve bu tür soruları kritik konu olarak ekibe devretmesini belirlersiniz. Yayına almadan önce test konsolunda örnek sorularla deneyebilirsiniz.",
      },
      {
        q: "Hastanın onayını nasıl kaydederiz?",
        a: "Her kişinin pazarlama ve hizmet izni ayrı tutulur; her değişiklikte kaynak, zaman ve değiştiren kişi kaydedilir. Onayı panelden, içe aktarmayla ya da API ile işleyebilirsiniz.",
      },
      {
        q: "Doktorlar hastalarla kendi telefonlarından mı yazışır?",
        a: "Hayır. Kliniğin numarası Messora'ya bağlanır, ekip panelden yazar. Konuşmalar kliniğin hesabında kalır; çalışan ayrıldığında kullanıcısını devre dışı bırakır, konuşmaları başka birine atarsınız.",
      },
    ],
    features: ["templates", "ai", "automations", "inbox"],
  },
  {
    key: "education",
    slug: "egitim",
    nav: "Eğitim",
    icon: "education",
    seoTitle: "Eğitim Kurumları için WhatsApp",
    seoDescription: "Okul, kurs ve eğitim kurumları için WhatsApp: kayıt dönemi sorularına yapay zeka ile cevap, aday takibi, ders hatırlatmaları ve velilerle izinli iletişim.",
    eyebrow: "Eğitim",
    h1: "Eğitim kurumları için WhatsApp: kayıt dönemini ve veli iletişimini düzene sokun",
    lead: "Kayıt döneminde gelen yüzlerce soruya ilk cevabı yapay zeka versin, adayın bilgileri konuşurken toplansın, kayıt ekibi yalnızca ciddi adaylarla görüşsün. Ders ve sınav hatırlatmaları velilere onaylı şablonla gitsin.",
    summary: "Kayıt dönemi sorularına ilk cevap, aday takibi, ders hatırlatmaları ve velilerle izinli iletişim.",
    pains: [
      {
        title: "Kayıt döneminde mesajlar yetişilemiyor",
        body: "Program, ücret ve takvim soruları aynı haftalarda yığılır. Cevap gecikince aday başka bir kuruma yazar.",
      },
      {
        title: "Adaylar takipte kayboluyor",
        body: "Bilgi alan aday bir daha aranmaz ya da üç kişi aynı adayı arar. Kimin hangi aşamada olduğu bir tabloda, o da güncel değil.",
      },
      {
        title: "Veli duyuruları dağınık",
        body: "Ders değişikliği, deneme sınavı ya da veli toplantısı duyurusu farklı gruplardan gider. Kimin okuduğu, kimin hiç almadığı bilinmez.",
      },
    ],
    flows: [
      {
        title: "Kayıt dönemi soruları",
        trigger: "Aday öğrenci ya da veli ilk kez yazdığında",
        steps: [
          "\"Yeni müşteri karşılama\" akışı karşılar ve \"yeni-musteri\" etiketi ekler",
          "Yapay zeka ajanı program, takvim ve ücret sorularını talimatlarınızdaki bilgilerle cevaplar",
          "Ad, e-posta ve ilgilendiği programı konuşma içinde sorar",
          "Bilgiler tamamlanınca konuşma özetle kayıt ekibine devredilir",
        ],
        outcome: "Kayıt ekibi her adayı, ne istediğini bilerek ve bilgileri hazır olarak karşılar.",
        features: ["automations", "ai", "inbox"],
      },
      {
        title: "Ders ve sınav hatırlatması",
        trigger: "Deneme sınavından ya da dersten bir gün önce",
        steps: [
          "Velileri sınıfa göre gruplara ayırın",
          "Bilgilendirme (utility) kategorisindeki hatırlatma şablonuyla gruba kampanya oluşturun",
          "Teslim ve okunma oranlarını analiz merkezinde izleyin",
        ],
        outcome: "Hangi velinin hatırlatmayı aldığını ve okuduğunu görürsünüz.",
        features: ["campaigns", "templates", "analytics"],
      },
      {
        title: "Sessiz kalan adayın takibi",
        trigger: "Aday bilgi aldıktan sonra 3 gün yanıt vermediğinde",
        steps: [
          "\"Belirli süre cevap vermeyen müşterileri tespit etme\" akışı adaya \"sessiz\" etiketi ekler",
          "Pazarlama izni olan adaylar için \"Cevapsız müşteriyi takip etme\" akışı onaylı bir hatırlatma şablonu gönderir",
          "Yanıt veren aday kayıt ekibinden bir temsilciye atanır",
        ],
        outcome: "Hiçbir aday unutulmaz; takip yalnızca mesaj almayı kabul edenlere yapılır.",
        features: ["automations", "templates", "inbox"],
      },
    ],
    templates: [
      {
        name: "ders_hatirlatma",
        category: "utility",
        body: "Sayın {{1}}, {{2}} için {{3}} tarihindeki dersin hatırlatmasıdır. Saat: {{4}}. Bir değişiklik olursa size buradan bildireceğiz.",
      },
      {
        name: "kayit_donemi",
        category: "marketing",
        body: "Merhaba {{1}}, {{2}} dönemi kayıtlarımız başladı. Programlarımızı incelemek ve ön kayıt oluşturmak için aşağıdaki bağlantıyı kullanabilirsiniz. Mesaj almak istemiyorsanız DUR yazabilirsiniz.",
        buttons: ["Programları incele", "Danışmanla görüş"],
      },
    ],
    compliance: "Öğrencilerin önemli bir kısmı 18 yaşından küçükse iletişimi veli üzerinden kurun ve onayı veliden alın. Çocuğa ait bilgileri en aza indirin: mesajlarda not, devamsızlık ya da sağlık bilgisi paylaşmayın. Messora'nın yapay zeka profillerinde kimlik numarası gibi özel alanlar tanımlanamaz. Kayıt dönemi duyuruları ve tanıtım mesajları ticari elektronik iletidir; önceden onay ve İYS kaydı gerekir. Ders ve sınav hatırlatmaları gibi kayıtlı öğrenciye yönelik bilgilendirmeler bilgilendirme (utility) şablonudur. Meta pazarlama ve bilgilendirme mesajlarını farklı ücretlendirir; şablonun kategorisini içeriğine göre seçin.",
    faq: [
      {
        q: "Velilere toplu duyuru gönderebilir miyiz?",
        a: "Evet. Velileri gruplara ayırıp onaylı bir şablonla kampanya gönderirsiniz. Ders değişikliği gibi bilgilendirmeler bilgilendirme (utility), kayıt ve tanıtım duyuruları pazarlama kategorisindedir.",
      },
      {
        q: "Öğrenciler doğrudan yazarsa ne yapmalıyız?",
        a: "Konuşmayı yanıtlayabilirsiniz, ancak 18 yaşından küçük öğrencilere pazarlama mesajı göndermeyin ve kayıt gibi konularda veliyle iletişime geçin. Bu kuralı yapay zeka profilinin talimatlarına da yazın.",
      },
      {
        q: "Kayıt ekibimiz kaç kişi olabilir?",
        a: "Kullanıcı sınırı Başlangıç'ta 2, Büyüme'de 5, Pro'da 12; Kurumsal'da ihtiyaca göre belirlenir. Konuşmaları ekip içinde atayarak paylaştırırsınız.",
      },
      {
        q: "Yapay zeka ücret ve program bilgisini nereden biliyor?",
        a: "Ajanın profiline yazdığınız talimatlardan. Programları, ücretleri ve takvimi orada güncel tutarsınız; ajan bilmediği konularda konuşmayı ekibe devreder.",
      },
    ],
    features: ["ai", "automations", "campaigns", "inbox"],
  },
  {
    key: "travel",
    slug: "turizm-ve-otel",
    nav: "Turizm ve otel",
    icon: "plane",
    seoTitle: "Otel ve Turizm için WhatsApp Business API",
    seoDescription: "Otel, acente ve turizm işletmeleri için WhatsApp: rezervasyon onayı, varış öncesi bilgilendirme, gece sorularına yapay zeka cevabı ve misafir verisinde özen.",
    eyebrow: "Turizm ve otel",
    h1: "Turizm ve oteller için WhatsApp: rezervasyondan çıkışa kadar misafirle konuşun",
    lead: "Rezervasyon onayı ve varış bilgileri WhatsApp'tan gitsin, gece gelen fiyat sorularına ilk cevabı yapay zeka versin, satış ekibi sabah hazır bilgilerle başlasın. Misafirin kimlik bilgileri bu kanala girmesin.",
    summary: "Rezervasyon onayı, varış öncesi bilgi, gece sorularına ilk cevap ve konaklama sonrası geri bildirim.",
    pains: [
      {
        title: "Sorular her saatte geliyor",
        body: "Misafirler farklı saat dilimlerinden, gece yarısı fiyat ve müsaitlik sorar. Sabaha kadar cevap alamayan misafir başka bir tesisi seçer.",
      },
      {
        title: "Aynı bilgiler tekrar tekrar soruluyor",
        body: "Giriş saati, transfer, otopark, kahvaltı saatleri gibi sorular her rezervasyonda yeniden gelir. Resepsiyon aynı cevabı gün boyu yazar.",
      },
      {
        title: "Kimlik fotoğrafları sohbetlerde",
        body: "Misafirler pasaport ve kimlik fotoğraflarını WhatsApp'tan gönderiyor. Bu veriler kişisel telefonlarda ve sohbet geçmişlerinde kontrolsüz kalıyor.",
      },
    ],
    flows: [
      {
        title: "Rezervasyon onayı ve varış bilgisi",
        trigger: "Rezervasyon oluşturulduğunda",
        steps: [
          "Rezervasyon sisteminiz bilgileri Messora API'sine gönderir",
          "Bilgilendirme (utility) kategorisindeki onay şablonu rezervasyon numarasıyla gider",
          "Varıştan önce ikinci bir bilgilendirme şablonu transfer ve giriş bilgilerini iletir",
          "Misafir \"Transfer istiyorum\" butonuna dokunursa konuşma resepsiyona atanır",
        ],
        outcome: "Misafir gerekli bilgileri varıştan önce alır; resepsiyon yalnızca talepleri ele alır.",
        features: ["api", "templates", "automations", "inbox"],
      },
      {
        title: "Gece gelen fiyat ve müsaitlik soruları",
        trigger: "Satış ekibi çevrim dışıyken misafir yazdığında",
        steps: [
          "Yapay zeka ajanı oda tipleri, olanaklar ve iptal koşulları gibi soruları talimatlarınıza göre cevaplar",
          "Teklif için tarihleri ve kişi sayısını konuşma içinde sorar",
          "Bilgiler tamamlanınca konuşma özetle satış ekibine devredilir",
        ],
        outcome: "Gece yazan misafir cevapsız kalmaz; satış ekibi sabah teklif için gereken bilgilerle başlar.",
        features: ["ai", "inbox"],
      },
      {
        title: "Konaklama sonrası geri bildirim",
        trigger: "Misafir çıkış yaptıktan sonra",
        steps: [
          "Pazarlama izni olan misafirlere \"Memnun kaldım\" ve \"Sorun yaşadım\" butonlu şablonla kampanya gönderin",
          "Butonları olumlu ve olumsuz olarak eşleyin",
          "\"Olumsuz müşteri dönüşü\" akışı sorun yaşayan misafiri not düşerek ekibe devreder",
          "\"Olumlu müşteri dönüşü\" akışı memnun misafiri etiketleyip ilgili kişiye atar",
        ],
        outcome: "Sorun yaşayan misafirle hızla ilgilenir, memnun misafirleri ayrı bir listede toplarsınız.",
        features: ["campaigns", "automations", "analytics"],
      },
    ],
    templates: [
      {
        name: "rezervasyon_onayi",
        category: "utility",
        body: "Sayın {{1}}, {{2}} ile {{3}} tarihleri arasındaki rezervasyonunuz onaylandı. Rezervasyon numaranız: {{4}}. Giriş saati: {{5}}.",
      },
      {
        name: "varis_bilgisi",
        category: "utility",
        body: "Merhaba {{1}}, {{2}} tarihindeki varışınız için bilgiler: {{3}}. Sorularınızı bu sohbetten yazabilirsiniz.",
        buttons: ["Transfer istiyorum", "Soru sormak istiyorum"],
      },
      {
        name: "konaklama_degerlendirme",
        category: "marketing",
        body: "Merhaba {{1}}, {{2}} konaklamanız için teşekkür ederiz. Deneyiminizi bizimle paylaşır mısınız? Mesaj almak istemiyorsanız DUR yazabilirsiniz.",
        buttons: ["Memnun kaldım", "Sorun yaşadım"],
      },
    ],
    compliance: "Pasaport ve kimlik bilgilerinde veri minimizasyonu ilkesine uyun: misafirlerden kimlik fotoğrafını WhatsApp'tan istemeyin, bu bilgileri girişte ya da kendi güvenli sisteminizde alın. Gelen medya konuşma geçmişinde saklanır. Messora'nın yapay zeka profillerinde kimlik ve pasaport numarası gibi alanlar tanımlanamaz ve ajana bunları istememesi söylenir. Rezervasyon onayı ve varış bilgileri bilgilendirme (utility), tanıtım ve geri bildirim talepleri pazarlama şablonudur; pazarlama için misafirin önceden onayını alın ve İYS kaydını yapın. Meta kategorileri ve alıcının ülkesine göre farklı ücret alır; yabancı misafirlere giden mesajların maliyeti o ülkenin fiyatıyla hesaplanır.",
    faq: [
      {
        q: "Yabancı misafirlere kendi dillerinde mesaj gönderebilir miyiz?",
        a: "Evet. Her şablon bir dille onaya gönderilir; aynı şablonun İngilizce ya da başka dillerde sürümlerini oluşturabilirsiniz. Yapay zeka ajanının hangi dilde cevap vereceğini de profil talimatlarında belirlersiniz.",
      },
      {
        q: "Misafir pasaport fotoğrafı gönderirse ne olur?",
        a: "Gelen medya konuşma geçmişinde saklanır. Bu yüzden misafirlerinizden kimlik belgesini WhatsApp yerine giriş sırasında ya da kendi güvenli sisteminizde istemenizi öneririz. Gerekirse kişiyi ve mesaj geçmişini silebilirsiniz.",
      },
      {
        q: "Rezervasyon sistemimize bağlanabilir mi?",
        a: "Hazır bir rezervasyon sistemi entegrasyonu yok. Sisteminiz REST API ile şablon mesaj gönderir, webhook'larla gelen mesajları ve teslim durumlarını alır. API çağrıları Büyüme planından itibaren açıktır.",
      },
      {
        q: "Yurt dışındaki numaralara mesaj göndermenin maliyeti farklı mı?",
        a: "Evet. Meta ücreti alıcının ülkesine ve şablonun kategorisine göre belirler. Messora kampanya öncesi maliyeti her alıcının ülkesine göre hesaplar; fiyat kartında olmayan ülkeleri tahmin etmez, ayrıca gösterir.",
      },
    ],
    features: ["templates", "ai", "api", "inbox"],
  },
  {
    key: "realestate",
    slug: "emlak",
    nav: "Emlak",
    icon: "home",
    seoTitle: "Emlak Ofisleri için WhatsApp Business API",
    seoDescription: "Emlakçılar için WhatsApp: ilan sorularını yapay zekayla karşılayın, bütçe ve bölgeyi öğrenin, gösterim randevusu bildirin, portföyü izinli adaylara duyurun.",
    eyebrow: "Emlak",
    h1: "Emlak ofisleri için WhatsApp: ilan sorusunu nitelikli adaya dönüştürün",
    lead: "İlan sorularına ilk cevabı yapay zeka versin, bütçe ve bölge bilgisini konuşurken toplasın, danışmana hazır bir aday bıraksın. Konuşmalar danışmanın telefonunda değil, ofisin hesabında kalsın.",
    summary: "İlan sorularına ilk cevap, aday bilgisi toplama, gösterim randevusu bildirimi ve izinli portföy duyuruları.",
    pains: [
      {
        title: "Her ilan sorusu ciddi alıcı değil",
        body: "Portallardan gelen mesajların çoğu \"Fiyat ne kadar?\" ile başlar ve biter. Danışmanlar gerçek alıcıyı bulmak için günlerini ilk cevaplara harcar.",
      },
      {
        title: "Danışman ayrılınca müşteri de gidiyor",
        body: "Yazışmalar danışmanların kişisel telefonlarında. Danışman ofisten ayrıldığında müşteri geçmişi ve süren görüşmeler onunla birlikte gider.",
      },
      {
        title: "Toplu ilan duyurusu yasal risk",
        body: "Numarası olan herkese yeni ilan göndermek, onay yoksa ticari elektronik ileti kurallarına aykırıdır. Şikâyetler numaranın kalitesini de düşürür.",
      },
    ],
    flows: [
      {
        title: "İlan sorusundan nitelikli adaya",
        trigger: "Bir ilan hakkında ilk mesaj geldiğinde",
        steps: [
          "Yapay zeka ajanı ilan hakkındaki genel soruları talimatlarınıza göre cevaplar",
          "Bütçe, bölge ve oda sayısını konuşma içinde birer birer sorar",
          "Bilgiler tamamlanınca konuşma özet ve toplanan bilgilerle bir danışmana devredilir",
          "Danışman toplanan bilgileri kontrol eder ve tek tıkla doğrular",
        ],
        outcome: "Danışman her görüşmeye adayın ne aradığını bilerek başlar.",
        features: ["ai", "inbox"],
      },
      {
        title: "Gösterim randevusu",
        trigger: "Gösterim randevusu planlandığında",
        steps: [
          "Bilgilendirme (utility) kategorisindeki randevu şablonu adres, saat ve danışman adıyla gider",
          "Aday \"Saati değiştirmek istiyorum\" butonuna dokunursa konuşma danışmana atanır",
          "\"Temsilciye devir\" akışı \"yetkili\" yazan adayı doğrudan ekibe aktarır",
        ],
        outcome: "Randevu bilgileri tek mesajda gider, değişiklik talepleri doğru danışmana ulaşır.",
        features: ["templates", "automations", "inbox"],
      },
      {
        title: "Yeni portföy duyurusu",
        trigger: "Yeni bir ilan portföye eklendiğinde",
        steps: [
          "Bölge ve bütçe bilgisine göre bir segment oluşturun",
          "İlanları carousel pazarlama şablonuyla gösterin",
          "Ön kontrol yalnızca pazarlama izni olan adayları sayar",
          "\"Olumlu müşteri dönüşü\" akışı ilgilenen adayı danışmana atar",
        ],
        outcome: "Duyuru yalnızca izin veren ve kriterleri uyan adaylara gider.",
        features: ["campaigns", "templates", "automations"],
      },
    ],
    templates: [
      {
        name: "gosterim_randevusu",
        category: "utility",
        body: "Merhaba {{1}}, {{2}} tarihinde saat {{3}} için {{4}} adresindeki gösterim randevunuz oluşturuldu. Danışmanınız: {{5}}.",
        buttons: ["Saati değiştirmek istiyorum"],
      },
      {
        name: "yeni_portfoy",
        category: "marketing",
        body: "Merhaba {{1}}, aradığınız kriterlere uygun {{2}} bölgesinde yeni ilanlarımız var. Ayrıntıları aşağıdan inceleyebilirsiniz. Mesaj almak istemiyorsanız DUR yazabilirsiniz.",
        buttons: ["İlanları incele", "Danışmanla görüş"],
      },
    ],
    compliance: "Yeni ilan ve portföy duyuruları 6563 sayılı Kanun ve Ticari İletişim ve Ticari Elektronik İletiler Hakkında Yönetmelik kapsamında ticari elektronik iletidir. Bir ilan portalından size yazmış olmak pazarlama mesajı için onay sayılmaz: duyuru göndermeden önce adayın açık onayını alın, onayın kaynağını kaydedin ve İYS'ye bildirin. Messora pazarlama iznini kapatan ya da izni olmayan adaylara pazarlama şablonu göndermez, \"DUR\" yazanın iznini hemen kapatır. Gösterim randevusu bildirimi bilgilendirme (utility) şablonudur; Meta pazarlama ve bilgilendirme mesajlarını farklı ücretlendirir.",
    faq: [
      {
        q: "Danışman işten ayrılınca konuşmalar ne olur?",
        a: "Konuşmalar ofisin hesabında kalır. Danışmanın kullanıcısını devre dışı bırakır, konuşmalarını başka bir danışmana atarsınız. Atama geçmişi ve denetim kaydı saklanır.",
      },
      {
        q: "Portaldan bize yazanlara ilan gönderebilir miyiz?",
        a: "Sorduğu ilanla ilgili konuşmaya 24 saat içinde cevap verebilirsiniz. Başka ilanları duyurmak pazarlama mesajıdır ve adayın açık onayını gerektirir.",
      },
      {
        q: "Hangi danışmanın hangi adayla konuştuğunu görebilir miyiz?",
        a: "Evet. Her konuşmanın bir sorumlusu vardır; atama ve devir geçmişi konuşmada görünür. Yöneticiler tüm konuşmaları görür, danışmanlar varsayılan olarak kendilerine atananları.",
      },
      {
        q: "Yapay zeka fiyat pazarlığı yapar mı?",
        a: "Ajan profilinize yazdığınız kurallara göre davranır. Fiyat ve pazarlık konularını kritik konu olarak tanımlarsanız bu sorular doğrudan danışmana devredilir.",
      },
    ],
    features: ["ai", "inbox", "campaigns", "automations"],
  },
  {
    key: "services",
    slug: "hizmet-isletmeleri",
    nav: "Hizmet işletmeleri",
    icon: "wrench",
    seoTitle: "Hizmet İşletmeleri için WhatsApp Randevu",
    seoDescription: "Kuaför, oto servis ve temizlik işletmeleri için WhatsApp: randevu hatırlatma, iş tamamlandı bildirimi, cevapsız mesaj uyarısı ve hızlı fiyat cevabı.",
    eyebrow: "Hizmet işletmeleri",
    h1: "Hizmet işletmeleri için WhatsApp: randevu, bildirim ve fiyat sorusu tek yerde",
    lead: "Randevu hatırlatması kendiliğinden gitsin, iş bittiğinde müşteri haber alsın, sahadaki ekip cevap veremediğinde mesaj sahipsiz kalmasın. Kuaförden oto servise kadar randevuyla çalışan her işletme için.",
    summary: "Randevu hatırlatma, iş tamamlandı bildirimi, cevapsız mesaj uyarısı ve fiyat sorularına ilk cevap.",
    pains: [
      {
        title: "Randevular unutuluyor",
        body: "Müşteri randevusunu unutur, boş kalan saat geri gelmez. Hatırlatma için tek tek aramaya ya da mesaj atmaya kimsenin vakti yoktur.",
      },
      {
        title: "Ekip sahada, mesajlar bekliyor",
        body: "Usta işin başındayken telefona bakamaz. Fiyat soran müşteri saatlerce cevap alamayınca başka bir işletmeyi arar.",
      },
      {
        title: "\"Aracım hazır mı?\" diye aranmak",
        body: "İşin durumunu soran müşteriler telefonla arar. Her arama, ekibin işini bölen bir kesinti demek.",
      },
    ],
    flows: [
      {
        title: "Randevu hatırlatma",
        trigger: "Randevudan bir gün önce",
        steps: [
          "Randevu sisteminiz API ile ya da ertesi günün müşterilerini içeren bir grupla bilgiyi aktarır",
          "Bilgilendirme (utility) kategorisindeki hatırlatma şablonu \"Onaylıyorum\" ve \"Değiştirmek istiyorum\" butonlarıyla gider",
          "\"Değiştirmek istiyorum\" mesajıyla başlayan otomasyon konuşmayı bir temsilciye atar",
        ],
        outcome: "Müşteri randevusunu hatırlar, değişiklik isteyenler doğrudan ekibe ulaşır.",
        features: ["templates", "api", "automations"],
      },
      {
        title: "İş tamamlandı bildirimi",
        trigger: "Servis ya da işlem tamamlandığında",
        steps: [
          "İş takip sisteminiz durumu Messora API'sine gönderir",
          "Bilgilendirme (utility) kategorisindeki şablon tutarı ve teslim saatini iletir",
          "Müşterinin soruları gelen kutusuna düşer ve işi yapan kişiye atanır",
        ],
        outcome: "Müşteri durumu sormak için aramaz; ekip işini bölmeden çalışır.",
        features: ["api", "templates", "inbox"],
      },
      {
        title: "Cevapsız kalan mesaj uyarısı",
        trigger: "Müşterinin mesajı 30 dakika cevapsız kaldığında",
        steps: [
          "\"Belirli süre cevap yok\" tetikleyicisiyle bir otomasyon kurun",
          "Konuşmaya \"30 dakikadır cevap bekliyor\" iç notu düşün",
          "Konuşmayı ekibin ortak havuzuna devredin; ekipteki kişilere bildirim gider",
        ],
        outcome: "Hiçbir müşteri mesajı sahipsiz kalmaz.",
        features: ["automations", "inbox"],
      },
      {
        title: "Fiyat sorularına ilk cevap",
        trigger: "Müşteri hizmet ya da fiyat sorduğunda",
        steps: [
          "Yapay zeka ajanı hizmet listenizdeki bilgilerle genel fiyat aralığını ve süreci anlatır",
          "Kesin teklif için gereken bilgileri (hizmet türü, adres, uygun gün) sorar",
          "Bilgiler tamamlanınca konuşmayı ekibe devreder",
        ],
        outcome: "Müşteri ilk cevabı beklemeden alır; ekip teklif için gereken bilgilerle döner.",
        features: ["ai", "inbox"],
      },
    ],
    templates: [
      {
        name: "randevu_hatirlatma",
        category: "utility",
        body: "Merhaba {{1}}, yarın saat {{2}} için {{3}} randevunuz bulunuyor. Gelemeyecekseniz lütfen aşağıdan bildirin.",
        buttons: ["Onaylıyorum", "Değiştirmek istiyorum"],
      },
      {
        name: "is_tamamlandi",
        category: "utility",
        body: "Merhaba {{1}}, {{2}} işleminiz tamamlandı. Toplam tutar: {{3}}. Teslim almak için {{4}} saatine kadar uğrayabilirsiniz.",
      },
    ],
    compliance: "Randevu hatırlatması ve iş tamamlandı bildirimi, içinde tanıtım olmadığı sürece bilgilendirme (utility) şablonudur. Bu mesajlara indirim ya da yeni hizmet duyurusu eklerseniz Meta şablonu pazarlama olarak sınıflandırabilir; pazarlama mesajları müşterinin önceden onayını ve İYS kaydını gerektirir. Meta kategorileri farklı ücretlendirir. Müşterinin son mesajından sonraki 24 saat içinde serbest metinle cevap verebilirsiniz; 1 Ekim 2026'dan itibaren numara başına aylık 1.000 ücretsiz hizmet mesajını aşan hizmet mesajları da ücretlendirilir. Güncel kuralları Meta'nın resmi fiyat sayfasından kontrol edin.",
    faq: [
      {
        q: "Randevu sistemimiz yok, yine de kullanabilir miyiz?",
        a: "Evet. Ertesi günün müşterilerini bir gruba ekleyip hatırlatma şablonuyla kampanya gönderebilir ya da gelen kutusundan tek tek şablon mesaj atabilirsiniz. Tarih ve saat gibi değişkenleri kişi alanlarından doldurursunuz.",
      },
      {
        q: "Sahadaki ekip mesajları nasıl görür?",
        a: "Ekip panelden mesajları görür ve yanıtlar; yeni mesajlar sayfa yenilemeden gelir. Konuşmayı işi yapan kişiye atarsanız müşteri doğrudan onunla yazışır.",
      },
      {
        q: "Bir hatırlatma mesajı kaça mal olur?",
        a: "Bilgilendirme (utility) kategorisindeki mesajlar için 1 Nisan 2026 tarifesinde Türkiye'ye teslim edilen mesaj başı ücret yaklaşık 0,0009 USD'dir. Güncel fiyatı Meta'nın resmi sayfasından kontrol edin.",
      },
      {
        q: "Kaç çalışanımız aynı numarayı kullanabilir?",
        a: "Başlangıç'ta 2, Büyüme'de 5, Pro'da 12 kullanıcı; Kurumsal'da ihtiyaca göre. Hepsi aynı numaranın konuşmalarını panelden yanıtlar.",
      },
    ],
    features: ["templates", "automations", "inbox", "ai"],
  },
];
