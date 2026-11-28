import type { Feature } from './types';

export const features: Feature[] = [
  {
    key: "inbox",
    slug: "ekip-gelen-kutusu",
    nav: "Ekip gelen kutusu",
    icon: "inbox",
    seoTitle: "WhatsApp Ekip Gelen Kutusu ve Konuşma Atama",
    seoDescription: "WhatsApp konuşmalarını ekipçe yönetin: atama, iç notlar, müşteri paneli, izin durumu ve 24 saat kuralı tek ekranda. Resmi WhatsApp Business API üzerinde.",
    eyebrow: "Ekip gelen kutusu",
    h1: "WhatsApp ekip gelen kutusu: her konuşmanın bir sahibi olsun",
    lead: "Müşteri mesajları tek bir telefonda kaybolmasın. Her konuşma bir temsilciye atanır, ekip iç notlarla haberleşir, müşterinin geçmişi ve izin durumu yan panelde durur. Yeni mesajlar sayfayı yenilemeden gelir.",
    summary: "Konuşmaları ekibe dağıtın, iç notlarla haberleşin, müşterinin geçmişini ve izin durumunu aynı ekranda görün.",
    highlights: ["Atama ve konuşma sahipliği", "Gerçek zamanlı güncellenir", "24 saat kuralı uygulanır"],
    blocks: [
      {
        title: "Her konuşmanın bir sahibi var",
        body: "Konuşmayı kendinize alın ya da bir ekip arkadaşınıza atayın. Kimseye atanmamış konuşmalar ortak havuzda bekler. Temsilci rolü varsayılan olarak yalnızca kendisine atanan ve henüz atanmamış konuşmaları görür; sahip, yönetici ve müdür hepsini görür.",
        points: [
          "Atama ve durum geçmişi konuşmada saklanır",
          "Kapatılan konuşma, müşteri yeniden yazınca kendiliğinden açılır",
          "Atama değişiklikleri denetim kaydına yazılır",
        ],
      },
      {
        title: "Müşteriye gitmeyen iç notlar",
        body: "Ekip arkadaşınıza konuşmanın içinden not bırakın. İç notlar hiçbir koşulda müşteriye gönderilmez. \"AI'a not\" olarak işaretlediğiniz notlar yapay zeka ajanına talimat olur; ajan bunları müşteriye aktarmaz.",
      },
      {
        title: "Müşteri paneli",
        body: "Konuşmanın yanında müşterinin etiketleri, notları, grupları ve pazarlama ile hizmet mesajları için ayrı ayrı izin durumu görünür. Yapay zekanın konuşmadan topladığı bilgiler \"doğrulanmadı\" işaretiyle gelir; kontrol edip tek tıkla doğrularsınız.",
        points: [
          "Pazarlama ve hizmet izni ayrı tutulur",
          "İzin değişikliklerinin kaynağı ve zamanı kayıtlı",
          "Kişi bazında yapay zekayı kapatabilirsiniz",
        ],
      },
      {
        title: "Metin, medya ve şablon",
        body: "Metin yazın, görsel, video, ses ya da belge ekleyin; dosya türü ve boyut sınırlarını sistem gönderimden önce kontrol eder. Müşterinin 24 saatlik penceresi kapandıysa gelen kutusu sizi onaylı bir şablon mesaja yönlendirir.",
      },
      {
        title: "Kurallar sizin yerinize işler",
        body: "Meta'nın kuralına göre serbest metin yalnızca müşterinin son mesajından sonraki 24 saat içinde gönderilebilir; gelen kutusu bu süreyi konuşmada gösterir. \"DUR\" ya da \"İPTAL\" yazan müşterinin pazarlama izni hemen kapanır, olay konuşma geçmişine düşer. Mesaj almak istemediğini kendi cümleleriyle söyleyen müşteri için konuşmanın sorumlusuna bildirim gider.",
      },
      {
        title: "Yapay zeka ile aynı ekranda",
        body: "Yapay zeka öneri modundaysa her müşteri mesajı için bir cevap taslağı hazırlar; siz kabul eder, düzenler ya da reddedersiniz. Yapay zeka bir konuşmayı devrettiğinde konuşma özetiyle birlikte \"İnsan bekleyen\" listesine düşer. Bir temsilci yazdığı anda konuşmayı devralır.",
      },
    ],
    steps: [
      { title: "Numaranızı bağlayın", body: "Meta'daki WhatsApp Business hesabınızı ve numaranızı bağlayın. Gelen mesajlar doğrudan gelen kutusuna düşer." },
      { title: "Ekibinizi davet edin", body: "Her kişiye sahip, yönetici, müdür ya da temsilci rolü verin. İsterseniz iki adımlı doğrulamayı açın." },
      { title: "Konuşmaları dağıtın", body: "Gelen konuşmaları kendinize alın ya da atayın; otomasyonlarla belirli konuşmaları doğrudan ilgili kişiye yönlendirin." },
      { title: "Takip edin ve kapatın", body: "İş bitince konuşmayı kapatın. Müşteri tekrar yazdığında konuşma geçmişiyle birlikte yeniden açılır." },
    ],
    plans: "Gelen kutusu her planda var. Kullanıcı sınırı Başlangıç'ta 2, Büyüme'de 5, Pro'da 12; Kurumsal'da ihtiyaca göre belirlenir. Pro planda 3 WhatsApp numarasının konuşmalarını aynı gelen kutusunda yönetirsiniz.",
    faq: [
      {
        q: "Aynı WhatsApp numarasını birden fazla kişi kullanabilir mi?",
        a: "Evet. Numara Messora'ya bağlanır ve planınızdaki kullanıcı sınırı kadar kişi aynı numaranın konuşmalarını panelden yanıtlar. Kimsenin telefonu paylaşması gerekmez.",
      },
      {
        q: "Temsilciler birbirinin konuşmalarını görür mü?",
        a: "Varsayılan olarak hayır. Temsilci kendisine atanan ve henüz atanmamış konuşmaları görür. Tüm konuşmaları görme yetkisi sahip, yönetici ve müdür rollerindedir.",
      },
      {
        q: "24 saat dolduysa müşteriye nasıl yazarım?",
        a: "Meta onaylı bir şablon mesajla. Pencere dışında serbest metin gönderilemez; gelen kutusu bunu kontrol eder ve sizi şablon seçimine yönlendirir. Müşteri yanıt verdiğinde yeni bir 24 saatlik pencere açılır.",
      },
      {
        q: "Müşteri \"DUR\" yazarsa ne olur?",
        a: "Müşterinin pazarlama izni hemen kapanır ve olay konuşma geçmişine yazılır. İsterseniz otomatik bir onay mesajı gider. Bu kişi sonraki kampanyalarda otomatik olarak atlanır; konuşmaya cevap vermeye devam edebilirsiniz.",
      },
      {
        q: "Gelen kutusu kendi CRM sistemimizle çalışır mı?",
        a: "Evet. Yeni mesaj, atama ve kapanış olaylarını imzalı webhook'larla sisteminize gönderebilir, API ile konuşmaları okuyup yanıt yazabilirsiniz. API çağrıları Büyüme planından itibaren açıktır.",
      },
    ],
    related: ["ai", "automations", "templates"],
  },
  {
    key: "campaigns",
    slug: "whatsapp-kampanyalari",
    nav: "Kampanyalar",
    icon: "megaphone",
    seoTitle: "WhatsApp Toplu Mesaj ve Kampanya Yönetimi",
    seoDescription: "Resmi API ile WhatsApp toplu mesaj gönderin: izin kontrolü, göndermeden önce Meta maliyeti, süre tahmini, duraklatma ve kalite uyarısında otomatik durdurma.",
    eyebrow: "WhatsApp kampanyaları",
    h1: "WhatsApp toplu mesaj: kime gideceğini ve ne tutacağını göndermeden görün",
    lead: "Kitlenizi segment, grup, etiket ya da yüklediğiniz listeyle seçin. Messora göndermeden önce mesajın kaç kişiye gidebileceğini, kimin neden engellendiğini, Meta'nın ne kadar ücret alacağını ve gönderimin ne zaman biteceğini gösterir.",
    summary: "İzinli kitleye şablon mesaj gönderin; maliyeti, süreyi ve engellenen kişileri baştan görün.",
    highlights: ["Göndermeden önce Meta maliyeti", "İzin kontrolü her gönderimde", "Kalite uyarısında otomatik durur"],
    blocks: [
      {
        title: "Kitleyi istediğiniz gibi kurun",
        body: "Kampanyayı bir segmente, gruba, etikete, tüm kişilere ya da CSV veya Excel ile yüklediğiniz listeye gönderin. Aynı kişi kitleye iki kez girmez.",
        points: [
          "CSV ve XLSX içe aktarma: kolon eşleme, ilerleme ve hata raporu",
          "Dinamik segmentler kurallara göre güncel kitleyi verir",
          "Zamanlanmış gönderim, seçtiğiniz saat dilimiyle",
        ],
      },
      {
        title: "Kime gidemeyeceğini önceden bilin",
        body: "Ön kontrol kitlenizi seçtiğiniz şablonun kategorisine göre sayar: gönderilebilecek, uyarılı ve engellenen kişiler, engel nedenleriyle birlikte. Pazarlama iznini kapatmış, WhatsApp'ta işletmenizin mesajlarını durdurmuş ya da numarasına ulaşılamayan kişilere mesaj gitmez; bu engeli hiçbir ayar kaldıramaz.",
        points: [
          "Engellenenler nedenlerine göre ayrı ayrı sayılır",
          "İzni bilinmeyen kişiler için \"uyar\" ya da \"engelle\" politikası",
          "Gönderimden hemen önce izin bir kez daha kontrol edilir",
        ],
      },
      {
        title: "Maliyeti göndermeden görün",
        body: "Meta ücreti, Meta'nın fiyat kartına göre her alıcının ülkesi ve şablonun kategorisi üzerinden hesaplanır. Fiyat kartında bulunmayan bir ülke tahmin edilmez, ayrı bir satırda \"fiyat kartında yok\" olarak gösterilir. Meta'nın mesaj ücretleri plan ücretinizden ayrıdır.",
      },
      {
        title: "Ne zaman biteceğini bilin, tahmin uydurmasın",
        body: "Ön analiz gönderimin süresini ve bitiş zamanını numaranızın gönderim kapasitesine ve aynı numarada çalışan diğer kampanyalara göre hesaplar; Meta'nın mesajlaşma limiti yetmiyorsa gönderimin kaç 24 saatlik dilime yayılacağını söyler. Beklenen okunma ve yanıt oranlarını yalnızca aynı kategoride en az 3 tamamlanmış kampanya ve 500 alıcılık geçmişiniz varsa gösterir; yoksa açıkça \"yeterli geçmiş yok\" der.",
      },
      {
        title: "Bir numara, birkaç kampanya, adil sıra",
        body: "Aynı numaradan çalışan kampanyalar gönderim kapasitesini önceliklerine göre adil biçimde paylaşır; gelen kutusundaki cevaplarınız kampanya yükü yüzünden gecikmez. Kampanya çalışırken önceliğini ve hız sınırını değiştirebilirsiniz.",
        points: [
          "Duraklatın: gönderilmemiş mesajlar bekletilir",
          "Devam ettirmeden önce kime gideceğini ve süreyi görün",
          "İptal edin: kalan mesajlar hiç gönderilmez",
        ],
      },
      {
        title: "Numaranızı koruyan otomatik durdurma",
        body: "Meta spam sınırı hatası verdiğinde, hesabınıza kısıtlama geldiğinde, numara ya da şablon kalitesi kırmızıya düştüğünde, Meta şablonu duraklattığında veya mesaj istemeyenlerin oranı eşiği aştığında kampanya kendiliğinden duraklar. Hangi kuralın hangi değerle tetiklendiğini görür, kontrol ettikten sonra devam ettirirsiniz.",
      },
    ],
    steps: [
      { title: "Şablonu seçin", body: "Meta'nın onayladığı bir şablon seçin ve değişkenleri kişi bilgileriyle eşleyin." },
      { title: "Kitleyi belirleyin", body: "Segment, grup, etiket ya da yüklenen liste seçin; ön kontrol izin durumuna göre sayar." },
      { title: "Ön analizi okuyun", body: "Gönderilebilecek ve engellenen kişileri, Meta maliyetini, süreyi ve bitiş zamanını inceleyin." },
      { title: "Başlatın ve izleyin", body: "Hemen başlatın ya da zamanlayın. Teslim, okunma ve yanıtlar canlı güncellenir." },
    ],
    plans: "Kampanyalar ve izin yönetimi her planda var. Kişi sayısı tüm planlarda sınırsızdır, Başlangıç'ta adil kullanım koşuluyla. Başlangıç ve Büyüme'de 1, Pro'da 3 WhatsApp numarasından gönderirsiniz. Meta'nın mesaj başı ücretleri plan ücretine dahil değildir ve göndermeden önce gösterilir.",
    faq: [
      {
        q: "WhatsApp'tan toplu mesaj göndermek kurallara uygun mu?",
        a: "Resmi WhatsApp Business API ile, Meta'nın onayladığı şablonla ve izin vermiş kişilere göndermek Meta kurallarına uygundur. Türkiye'de pazarlama mesajları ticari elektronik ileti sayılır; alıcının önceden onayı ve İYS kaydı gerekir. Messora izin kayıtlarını tutar, İYS kaydı sizin sorumluluğunuzdadır.",
      },
      {
        q: "Mesaj başına ne kadar öderim?",
        a: "Meta, teslim edilen şablon mesajlar için ülkeye ve kategoriye göre ücret alır. 1 Nisan 2026 tarifesinde Türkiye için pazarlama mesajı yaklaşık 0,0109 USD, bilgilendirme (utility) ve doğrulama mesajı yaklaşık 0,0009 USD'dir. Güncel fiyatı Meta'nın resmi fiyat sayfasından kontrol edin; Messora tutarı kampanyayı başlatmadan önce hesaplar.",
      },
      {
        q: "Kampanyayı yarıda durdurabilir miyim?",
        a: "Evet. Duraklattığınızda gönderilmemiş mesajlar bekletilir. Devam ettirmeden önce izinler yeniden kontrol edilerek kime gideceğini ve tahmini süreyi görürsünüz. İptal ederseniz kalan mesajlar hiç gönderilmez.",
      },
      {
        q: "Numaramın kalitesi düşerse ne olur?",
        a: "Kampanya koruması açıksa, ki varsayılan olarak açıktır, numara ya da şablon kalitesi kırmızıya düştüğünde kampanya kendiliğinden duraklar. Eşikleri Ayarlar'daki kampanya koruması bölümünden değiştirebilirsiniz.",
      },
      {
        q: "Excel listemi yükleyip gönderebilir miyim?",
        a: "Evet. CSV ya da XLSX dosyanızı yükler, kolonları eşlersiniz; hatalı satırlar raporlanır. Yeni eklenen kişilerin pazarlama izni \"bilinmiyor\" olarak başlar. İzni olmayan listelere pazarlama mesajı göndermeyin; izin bilgisini içe aktarırken ya da sonradan güncelleyebilirsiniz.",
      },
      {
        q: "Okunma oranını önceden söyleyebilir misiniz?",
        a: "Yalnızca elinizde yeterli geçmiş varsa: aynı kategoride en az 3 tamamlanmış kampanya ve toplam 500 alıcı. Bu geçmiş yoksa ön analiz oran uydurmaz, \"yeterli geçmiş yok\" der ve neyin eksik olduğunu gösterir.",
      },
    ],
    related: ["templates", "analytics", "automations"],
  },
  {
    key: "ai",
    slug: "yapay-zeka-ajanlari",
    nav: "AI ajanları",
    icon: "sparkle",
    seoTitle: "WhatsApp Yapay Zeka Ajanı ve Chatbot",
    seoDescription: "WhatsApp yapay zeka ajanı müşterilerinize cevap verir, bilgi toplar, emin olmadığında ekibinize devreder. OpenAI, Anthropic ve Gemini modelleriyle çalışır.",
    eyebrow: "Yapay zeka ajanları",
    h1: "WhatsApp yapay zeka ajanı: cevaplasın, emin olmadığında ekibinize devretsin",
    lead: "OpenAI, Anthropic veya Google Gemini modelleriyle çalışan bir ajan, işletmenizin talimatlarına göre müşterilerinize cevap verir. Müşteri bir insanla konuşmak istediğinde ya da ajan cevaptan emin olmadığında konuşmayı özetiyle birlikte ekibinize bırakır.",
    summary: "Müşteri sorularını yanıtlayan, bilgi toplayan ve gerektiğinde ekibe devreden yapay zeka ajanları.",
    highlights: ["OpenAI, Anthropic ve Gemini", "İstendiğinde insana devreder", "Konuşmalar eğitimde kullanılmaz"],
    blocks: [
      {
        title: "Üç çalışma biçimi",
        body: "Her WhatsApp numarasının bir AI profili olur: talimatlar, model seçimi ve devir kuralları. Ajanı ihtiyacınıza göre çalıştırın.",
        points: [
          "Otomatik cevap: ajan müşteriye doğrudan yazar",
          "Onaylı cevap: ajan yazar, temsilci onaylayınca gider",
          "Yalnızca öneri: ajan taslak hazırlar, cevabı siz verirsiniz",
        ],
      },
      {
        title: "İnsana devir her zaman açık",
        body: "Müşteri \"temsilci\", \"canlı destek\" ya da \"gerçek biriyle görüşmek istiyorum\" dediğinde devir kapatılamaz; Meta'nın politikası insana hızlı ve açık bir yol ister. Ajan ayrıca güveni düşük olduğunda, cevabı olmadığında, kritik bir konu algıladığında, yetkisi olmayan bir işlem gerektiğinde ve 24 saatlik pencere kapandığında devreder. Devirde ekibe konuşmanın özeti, müşterinin ihtiyacı, önerilen sonraki adım ve toplanan bilgiler gider.",
      },
      {
        title: "Müşteri bilgisini konuşurken toplar",
        body: "Ad, e-posta, şehir ya da tanımladığınız başka alanları ajan konuşmanın içinde, cevap verdikten sonra birer ikişer sorar; müşteriye form doldurtmaz. Bilgiler tamamlanınca konuşmayı satış ekibinize devredebilir.",
        points: [
          "Kaydedilen değerler \"AI tarafından toplandı, doğrulanmadı\" işaretlidir",
          "Bir kişinin girdiği ya da doğruladığı değerin üzerine yazmaz",
          "Sağlık bilgisi, kimlik ve kart numarası gibi alanlar tanımlanamaz",
        ],
      },
      {
        title: "Ekibinizden talimat alır",
        body: "Konuşmaya \"AI'a not\" olarak eklediğiniz iç notlar, ajanın sonraki cevaplarında talimat olarak kullanılır; ajan bu notları müşteriye aktarmaz. Ajanın hangi işlemleri yapabileceğini (kişi oluşturma, etiket ekleme, atama, otomasyon başlatma) profilde siz belirlersiniz.",
      },
      {
        title: "Yayına almadan deneyin, maliyeti görün",
        body: "Test konsolunda kayıtlı profili örnek bir konuşmayla çalıştırın; cevabı, güven değerini, harcanan token'ı ve maliyeti görürsünüz, müşteriye hiçbir şey gitmez. Her yapay zeka isteği kullanım kaydına yazılır. Saatlik cevap sınırı ve aylık bütçe dolduğunda ajan konuşmayı ekibe bırakır.",
      },
      {
        title: "Konuşmalarınız model eğitimine gitmez",
        body: "Konuşmalar yalnızca seçtiğiniz sağlayıcıya gönderilir. OpenAI isteklerinde yanıtın saklanmaması istenir (store: false); Anthropic ve Google ücretli API projelerinde verileri model eğitiminde kullanmaz. İlk yapay zeka mesajının başında müşteriye bir yapay zekayla konuştuğunu söyleyen açıklama metni yer alır.",
      },
    ],
    steps: [
      { title: "Yapay zekayı açın", body: "Ayarlar'dan yapay zekayı açın ve varsayılan sağlayıcıyla modeli seçin." },
      { title: "Profili yazın", body: "Numaranız için işletmenizi, kurallarınızı ve devir koşullarını anlatan talimatları girin." },
      { title: "Test konsolunda deneyin", body: "Örnek sorularla profili sınayın, cevapları ve maliyeti görün." },
      { title: "Yayına alın", body: "Öneri moduyla başlayın, sonuçlardan emin olunca otomatik cevaba geçin." },
    ],
    plans: "Başlangıç'ta 1, Büyüme'de 3, Pro'da 10 AI ajanı. Aylık AI kredisi Başlangıç'ta 300, Büyüme'de 2.000, Pro'da 8.000. Kurumsal'da özel kredi havuzu ve kendi AI anahtarlarınızı kullanma seçeneği var.",
    faq: [
      {
        q: "Hangi yapay zeka modellerini kullanabilirim?",
        a: "OpenAI, Anthropic (Claude) ve Google Gemini modelleri desteklenir; DeepSeek de bağlanabilir. Varsayılan modeli şirket genelinde seçer, istediğiniz numaranın profilinde başka bir modelle değiştirebilirsiniz.",
      },
      {
        q: "Yapay zeka müşteriye yanlış bir şey söylerse?",
        a: "Riski azaltmak için onaylı cevap modunu kullanın: ajan cevabı yazar, temsilci onaylamadan gönderilmez. Güven eşiğinin altında kalan cevaplar ve kritik konular ekibe devredilir. Profili yayına almadan önce test konsolunda deneyebilirsiniz.",
      },
      {
        q: "Müşteri bir insanla konuşmak isterse ne olur?",
        a: "Ajan konuşmayı hemen ekibe devreder. Bu kural kapatılamaz. Konuşma \"İnsan bekleyen\" listesine düşer, ilgili kişilere bildirim gider ve devralan temsilci özetle birlikte konuşmayı görür.",
      },
      {
        q: "Konuşmalarımız yapay zeka modellerini eğitmek için kullanılır mı?",
        a: "Hayır. Meta, WhatsApp verisiyle model eğitimini yasaklar. Konuşmalar yalnızca seçtiğiniz sağlayıcıya gider; OpenAI isteklerinde store: false kullanılır, Anthropic ve Google ücretli API verisini eğitimde kullanmaz.",
      },
      {
        q: "AI kredisi nasıl harcanır?",
        a: "Cevaplar, öneriler, devir özetleri, kampanya yanıtlarının sınıflandırılması ve otomasyonlardaki yapay zeka adımları kredinizden düşer. Her istek kullanım kaydına yazılır; kullanımı panelde amaç ve kampanya bazında görürsünüz.",
      },
      {
        q: "24 saatlik pencere kapandığında ajan cevap verir mi?",
        a: "Hayır. Pencere kapalıyken serbest metin gönderilemez; otomatik cevaptaki ajan bu durumda konuşmayı ekibe devreder. Müşteriye yeniden ulaşmak için onaylı bir şablon gerekir.",
      },
    ],
    related: ["inbox", "automations", "analytics"],
  },
  {
    key: "automations",
    slug: "otomasyonlar",
    nav: "Otomasyonlar",
    icon: "flow",
    seoTitle: "WhatsApp Otomatik Mesaj ve Akış Otomasyonu",
    seoDescription: "WhatsApp otomatik mesaj akışlarını görsel editörde kurun: karşılama, temsilciye devir, cevapsız müşteri takibi ve kampanya sonrası işler için 11 hazır şablon.",
    eyebrow: "Otomasyonlar",
    h1: "WhatsApp otomatik mesaj: tekrar eden işleri akışlara bırakın",
    lead: "İlk kez yazan müşteriyi karşılayın, \"temsilci\" diyeni ekibe aktarın, cevapsız kalanı takip edin. Akışları görsel bir tuvalde tetikleyici ve adımlarla kurarsınız; her çalışmanın izlediği yol tuvalde görünür.",
    summary: "Görsel editörde tetikleyici ve adımlarla akış kurun; hazır şablonlarla dakikalar içinde başlayın.",
    highlights: ["Görsel akış editörü", "11 hazır otomasyon", "Döngü ve tekrar koruması"],
    blocks: [
      {
        title: "Görsel tuvalde kurun",
        body: "Akış bir tetikleyiciyle başlar, adımlarla devam eder. Koşul adımları evet ve hayır yollarına ayrılır, iç içe dört seviyeye kadar dallanır. Geri alma, yineleme, otomatik yerleşim ve mini harita büyük akışlarda yolunuzu kaybetmemenizi sağlar.",
      },
      {
        title: "Ne zaman başlasın",
        body: "Tetikleyiciler WhatsApp'taki gerçek olaylara bağlıdır.",
        points: [
          "Mesaj geldiğinde (anahtar kelimeyle ya da her mesajda), ilk kez yazan kişi",
          "Etiket eklendiğinde, konuşma kapandığında, mesaj teslim edildi, okundu ya da başarısız olduğunda",
          "Abonelik talebi, 24 saatlik pencere kapanmak üzereyken, belirli süre cevap gelmediğinde",
          "Kampanyaya ilk yanıt, olumlu ya da olumsuz dönüş, yapay zekanın ekibe devri",
        ],
      },
      {
        title: "Ne yapsın",
        body: "Cevap yazma, şablon gönderme, etiket ekleme ve kaldırma, gruba ekleme, alan güncelleme, bekleme (30 güne kadar), atama, konuşmayı kapatma, sessize alma, iç not, HTTP isteği ve e-posta. Yapay zeka adımları niyet tespit eder, bilgi çıkarır, duygu analizi yapar, kişiyi bir segmente yerleştirir, mesaj taslağı üretir ya da konuşmayı ajana veya bir temsilciye devreder.",
      },
      {
        title: "Hazır otomasyonlar",
        body: "Sık ihtiyaçlar için hazır akışlar her planda var. Kurduğunuz kopya sizindir; istediğiniz gibi düzenlersiniz. Eksik ayarları (örneğin hangi şablonun gideceği) doldurmadan etkinleşmez.",
        points: [
          "Yeni müşteri karşılama, Abonelikten çıkma talebi, Temsilciye devir",
          "Müşteri bilgisi toplama (AI), AI ile otomatik cevap",
          "24 saatlik pencere hatırlatması, Cevapsız müşteriyi takip etme",
          "Kampanya sonrası takip, Olumlu müşteri dönüşü, Olumsuz müşteri dönüşü",
          "Belirli süre cevap vermeyen müşterileri tespit etme",
        ],
      },
      {
        title: "Kendini tekrarlamaz, kural çiğnemez",
        body: "Her olay bir akışı en fazla bir kez başlatır; bir otomasyonun yaptığı değişiklik başka bir otomasyonu tetiklemez. Kişi başına bekleme süresi tanımlayabilirsiniz. Mesaj adımı yeniden denense bile aynı mesaj iki kez gitmez. İzin ve 24 saat kuralları burada da geçerlidir: uygulanamayan adım \"atlandı\" olarak kaydedilir, akış devam eder.",
      },
      {
        title: "Her çalışmanın kaydı",
        body: "Her çalışma ve her adım kaydedilir; çalışmanın izlediği yol tuvalde işaretlenir. Bir adım hata verirse nedeni yazar. HTTP, e-posta, mesaj ve yapay zeka adımlarına yeniden deneme ekleyebilirsiniz; geçici hatalarda adım bekleyip tekrar çalışır.",
      },
    ],
    steps: [
      { title: "Başlangıç noktasını seçin", body: "Hazır otomasyonlardan birini kurun ya da boş bir tuvalle başlayın." },
      { title: "Tetikleyiciyi belirleyin", body: "Akışın hangi olayla, hangi numarada ve hangi koşulla başlayacağını seçin." },
      { title: "Adımları ekleyin", body: "Mesaj, etiket, koşul, bekleme ve devir adımlarını tuvale sürükleyip bağlayın." },
      { title: "Etkinleştirin ve izleyin", body: "Akışı açın; çalışma geçmişini ve adım kayıtlarını tuvalde takip edin." },
    ],
    plans: "Başlangıç'ta 3, Büyüme'de 20 aktif akış; Pro ve Kurumsal'da sınırsız. Hazır şablonlar her planda. Yapay zeka adımları planınızın aylık AI kredisinden düşer.",
    faq: [
      {
        q: "Otomasyon kurmak için kod bilmem gerekir mi?",
        a: "Hayır. Akışları görsel tuvalde düğümleri bağlayarak kurarsınız. Hazır otomasyonlardan biriyle başlayıp yalnızca metinleri ve etiketleri değiştirmeniz çoğu zaman yeterlidir.",
      },
      {
        q: "Otomasyon müşteriye istemediği bir mesaj gönderebilir mi?",
        a: "Hayır. Otomasyonlar da kampanyalarla aynı izin kurallarına tabidir: pazarlama iznini kapatan kişiye pazarlama şablonu gitmez, 24 saatlik pencere kapalıyken serbest metin gönderilmez. Uygulanamayan adım atlanır ve kayda geçer.",
      },
      {
        q: "24 saatlik pencere hatırlatması müşteriye mesaj mı atar?",
        a: "Hayır. Bu hazır akış pencere kapanmadan önce ekibinize not düşer ve konuşmayı bir temsilciye yönlendirir. Pencereyi açık tutmak için müşteriye boş mesaj göndermek Meta kurallarına aykırıdır; hiçbir hazır akış bunu yapmaz.",
      },
      {
        q: "Otomasyonlar dış sistemlerimize bağlanabilir mi?",
        a: "Evet. HTTP isteği adımı HTTPS adreslerine istek gönderir; özel ağ adreslerine istek engellenir. E-posta adımı belirlediğiniz adrese bilgi gönderir. Olayları ayrıca imzalı webhook'larla sisteminize aktarabilirsiniz.",
      },
      {
        q: "Bir akışın neden çalışmadığını nasıl anlarım?",
        a: "Çalışma geçmişi her adımın sonucunu gösterir: tamamlandı, atlandı ya da hata verdi, nedeniyle birlikte. Çalışmanın izlediği yol tuvalde işaretlenir; yapay zeka kapalıysa ilgili düğümler uyarı rozeti taşır.",
      },
    ],
    related: ["ai", "inbox", "campaigns"],
  },
  {
    key: "templates",
    slug: "sablon-olusturucu",
    nav: "Şablon oluşturucu",
    icon: "template",
    seoTitle: "WhatsApp Şablon Mesaj Oluşturucu",
    seoDescription: "WhatsApp şablon mesajlarını panelde hazırlayın: medya başlığı, butonlar, değişkenler, carousel, süreli teklif ve doğrulama kodu. Meta onayını buradan izleyin.",
    eyebrow: "Şablon oluşturucu",
    h1: "WhatsApp şablon mesaj: kuralları yazarken kontrol eden oluşturucu",
    lead: "Metin ya da medya başlıklı şablonları, butonları, değişkenleri, carousel'i, süreli teklifi ve doğrulama kodu şablonlarını panelde hazırlayın. Messora, Meta'nın belgelediği sınırları siz yazarken kontrol eder, şablonu onaya gönderir ve onay durumunu takip eder.",
    summary: "Meta'nın sınırlarını yazarken kontrol eden şablon oluşturucu; onaya gönderin, durumunu buradan izleyin.",
    highlights: ["Yazarken sınır kontrolü", "Onay durumu otomatik güncellenir", "Carousel ve süreli teklif"],
    blocks: [
      {
        title: "Başlık, gövde, altbilgi ve butonlar",
        body: "Başlık metin, görsel, video, belge ya da konum olabilir. Gövde 1.024 karaktere kadar çıkar ve {{1}}, {{2}} gibi değişkenler taşır; her değişken için Meta'nın istediği örnek değeri girersiniz. Bir şablona en fazla 10 buton eklenir.",
        points: [
          "Şablon butonları: hızlı yanıt butonu, web bağlantısı, telefon, kod kopyalama ve sesli arama",
          "Medya başlığının örnek dosyası Meta'ya oluşturucudan yüklenir",
          "Karakter ve buton sınırları yazarken gösterilir",
        ],
      },
      {
        title: "Carousel",
        body: "Pazarlama şablonlarında 2 ile 10 arasında kaydırılabilir kart gösterin. Her kartın görseli ya da videosu, isteğe bağlı kısa metni ve en fazla iki butonu olur; kartların yapısı aynıdır.",
      },
      {
        title: "Süreli teklif",
        body: "WhatsApp'ın geri sayım gösterdiği süreli teklif şablonunu kupon kodu kopyalama ve bağlantı butonuyla hazırlayın. Teklifi başlık, indirim, kupon ve bitiş tarihiyle bir kez tanımlarsınız; kampanya bu bilgileri şablona kendisi yerleştirir. Teklif bittiğinde ona bağlı kampanya kendiliğinden duraklar.",
      },
      {
        title: "Doğrulama kodu (OTP)",
        body: "Giriş ve doğrulama için authentication kategorisinde şablon oluşturun. Mesaj metnini Meta belirler; siz kod kopyalama, tek dokunuşla doldurma ya da otomatik doldurma butonunu ve kodun 1 ile 90 dakika arasındaki geçerlilik süresini seçersiniz. Kodu API ile gönderirsiniz.",
      },
      {
        title: "WhatsApp Flows",
        body: "Meta'da hazırladığınız bir WhatsApp Flow'u şablona buton olarak bağlayın; müşteri form ya da randevu ekranını WhatsApp içinde açar. Flow ekranlarının tasarımı Meta tarafında yapılır.",
      },
      {
        title: "Onay ve durum takibi",
        body: "Şablonu Meta'ya oluşturucudan gönderin. Onay, ret ya da duraklatma gibi durum değişiklikleri ve kalite puanı Messora'ya kendiliğinden gelir; WhatsApp Business hesabınızdaki mevcut şablonlar da senkronize edilir. Meta bir şablonu duraklatırsa onu kullanan kampanya da durur.",
      },
    ],
    steps: [
      { title: "Kategori ve dili seçin", body: "Pazarlama, bilgilendirme (utility) ya da doğrulama kategorisini ve şablonun dilini belirleyin." },
      { title: "İçeriği hazırlayın", body: "Başlığı, gövdeyi ve butonları yazın, değişkenlere örnek değer girin; sınırlar anında kontrol edilir." },
      { title: "Onaya gönderin", body: "Şablonu Meta'ya gönderin; onay durumu panelde kendiliğinden güncellenir." },
      { title: "Her yerde kullanın", body: "Onaylanan şablonu kampanyalarda, gelen kutusunda, otomasyonlarda ve API'de kullanın." },
    ],
    plans: "Şablon oluşturucu ve süreli teklifler her planda var. Şablonların onayı, kalite puanı ve hesap başına şablon sınırları Meta tarafından belirlenir.",
    faq: [
      {
        q: "Şablon mesaj neden gerekli?",
        a: "Meta, işletmenin başlattığı ve müşterinin son mesajından 24 saat sonra gönderilen her mesajın onaylı bir şablon olmasını ister. Pencere içinde serbest metin yazabilirsiniz; kampanya, hatırlatma ve bildirimler için şablon gerekir.",
      },
      {
        q: "Hangi kategoriyi seçmeliyim?",
        a: "Tanıtım, indirim ve yeni ürün duyuruları pazarlama; sipariş, randevu ve hesap bildirimleri bilgilendirme (utility); tek kullanımlık kodlar doğrulama (authentication) kategorisidir. Meta kategorileri farklı ücretlendirir ve içeriğe uymayan kategoriyi değiştirebilir.",
      },
      {
        q: "Şablonum reddedilirse ne yaparım?",
        a: "Ret durumu panelde görünür. İçeriği Meta'nın kurallarına göre düzeltip şablonu yeniden oluşturur ve tekrar onaya gönderirsiniz. Değişkenlerin örnek değerleri ve kategori seçimi en sık karşılaşılan ret nedenleri arasındadır.",
      },
      {
        q: "Onay ne kadar sürer?",
        a: "Süreyi Meta belirler. Durum değiştiği anda Meta bunu Messora'ya bildirir ve panel kendiliğinden güncellenir; sayfayı yenilemeniz ya da Meta'ya ayrıca bakmanız gerekmez.",
      },
      {
        q: "Değişkenleri her müşteriye göre nasıl doldururum?",
        a: "Kampanyada her değişkeni kişinin adı, telefonu, e-postası, ek alanları ya da özel alanlarıyla eşlersiniz. API ile gönderirken değerleri istekte verirsiniz.",
      },
    ],
    related: ["campaigns", "automations", "api"],
  },
  {
    key: "analytics",
    slug: "analitik-ve-raporlar",
    nav: "Analitik ve raporlar",
    icon: "chart",
    seoTitle: "WhatsApp Kampanya Analitiği ve Raporları",
    seoDescription: "Her WhatsApp kampanyası için teslim, okunma, yanıt, olumlu ve olumsuz dönüş, abonelikten çıkma ve maliyeti görün. Tahmini gerçekle karşılaştırın, rapor alın.",
    eyebrow: "Analitik ve raporlar",
    h1: "WhatsApp kampanya raporları: kim okudu, kim yanıtladı, kaça mal oldu",
    lead: "Her kampanyanın teslim, okunma, yanıt, olumlu ve olumsuz dönüş, abonelikten çıkma ve maliyet rakamlarını tek ekranda görün. Her rakamın yanında kaynağı yazar: Meta'nın bildirdiği mi, sistemin hesapladığı mı.",
    summary: "Kampanya sonuçlarını kaynağı belli rakamlarla görün; kırılımlara bakın, tahminle karşılaştırın, raporu indirin.",
    highlights: ["Kaynağı belli rakamlar", "Tahmin ile gerçek yan yana", "Excel, CSV ve PDF rapor"],
    blocks: [
      {
        title: "Kampanya analiz merkezi",
        body: "Kitle, engellenen, gönderilen, teslim edilen, okunan, başarısız, yanıtlayan, olumlu, olumsuz ve abonelikten çıkan kişiler; oranlar ve maliyet. Rakamlar kampanya sürerken canlı güncellenir. Teslim ve okunma Meta'dan gelir; yanıt, tepki ve maliyet \"Sistem hesaplaması\" olarak işaretlenir.",
      },
      {
        title: "Yanıt doğru kampanyaya yazılır",
        body: "Bir müşterinin yanıtı ya da abonelikten çıkması, ona en son ulaşan kampanyaya sayılır; yalnızca kampanyanın gönderildiği numaraya ve belirlediğiniz süre içinde (varsayılan 7 gün, 1 ile 30 gün arası) gelirse. Otomasyonlar da aynı kuralı kullanır, bu yüzden raporla akışlar hep aynı kampanyayı gösterir.",
      },
      {
        title: "Olumlu ve olumsuz dönüşler",
        body: "Şablonunuzdaki hızlı yanıt (quick reply) butonlarını olumlu, olumsuz ya da nötr olarak eşleyin; müşteri dokunduğunda etiket kendiliğinden yazılır. Serbest metin yanıtlarını isterseniz yapay zeka sınıflandırır. Bir temsilcinin elle verdiği etiket sistem tarafından değiştirilmez.",
      },
      {
        title: "Kırılımlar",
        body: "Aynı rakamları grup, segment, etiket, kişi türü ya da bir özel alanın değerine göre ayrı ayrı görün. Hangi kitlenin daha çok yanıt verdiğini ya da daha çok abonelikten çıktığını bir bakışta karşılaştırırsınız.",
      },
      {
        title: "Tahmin ile gerçek",
        body: "Kampanya başlarken ön analiz sabitlenir. Kampanya ilerledikçe gerçek rakamlar tahminin yanında, farkıyla birlikte gösterilir. Tahmin yalnızca yeterli geçmiş olduğunda oran içerir; geçmiş yoksa oran uydurulmaz.",
      },
      {
        title: "Alıcı tablosu ve dışa aktarma",
        body: "Alıcıları duruma, etikete, gruba, segmente ya da özel alana göre süzün; seçtiklerinize etiket ekleyin, gruba ekleyin ya da onlar için bir takip kampanyası taslağı oluşturun. Süzdüğünüz listeyi CSV ya da Excel olarak indirin.",
        points: [
          "Excel raporu: Özet, Maliyet, Teslimat, Alıcılar, Segment, Etiket, Tahmin vs Gerçek sayfaları",
          "Tek sayfalık PDF özet",
          "Büyük dışa aktarmalar arka planda hazırlanır, 7 gün saklanır",
        ],
      },
    ],
    steps: [
      { title: "Kampanyayı başlatın", body: "Ön analiz kampanya başlarken sabitlenir ve sonradan gerçekle karşılaştırılır." },
      { title: "Tepkileri eşleyin", body: "Şablondaki hızlı yanıt butonlarını olumlu ya da olumsuz olarak işaretleyin." },
      { title: "Sonuçları okuyun", body: "Analiz merkezinde oranlara, maliyete ve kırılımlara bakın." },
      { title: "Harekete geçin", body: "Yanıt vermeyenleri ya da olumlu dönenleri seçin, etiketleyin veya takip kampanyası hazırlayın." },
    ],
    plans: "Analiz merkezi, kırılımlar ve dışa aktarma kampanyalarla birlikte her planda gelir. Serbest metin yanıtlarının yapay zekayla sınıflandırılması planınızın aylık AI kredisinden düşer.",
    faq: [
      {
        q: "Beni engelleyen ya da şikâyet eden müşteriyi görebilir miyim?",
        a: "Hayır. Meta, işletmeyi hangi kişinin engellediğini ya da şikâyet ettiğini bildirmez. Messora toplu sinyalleri gösterir: numara ve şablon kalitesi, Meta'nın hata kodları ve abonelikten çıkma oranı.",
      },
      {
        q: "Bir yanıt hangi kampanyaya sayılır?",
        a: "Müşteriye en son ulaşan kampanyaya; yanıt aynı numaraya ve belirlediğiniz süre içinde (varsayılan 7 gün) gelmişse. Daha yeni bir kampanya müşteriye ulaştığında önceki kampanyanın süresi kapanır.",
      },
      {
        q: "Maliyet rakamı Meta faturasıyla aynı mı?",
        a: "Messora maliyeti Meta'nın fiyat kartına göre hesaplar ve \"Sistem hesaplaması\" olarak gösterir; Meta'nın kendi günlük maliyet verisi de ayrıca alınır. Kesin tutar Meta faturanızdadır.",
      },
      {
        q: "Rakamlar ne sıklıkla güncellenir?",
        a: "Canlı. Teslim, okunma ve yanıtlar geldikçe ekran sayfa yenilemeden güncellenir. Sayaçlar ayrıca birkaç dakikada bir satırların kendisinden yeniden hesaplanıp kontrol edilir.",
      },
      {
        q: "Raporu yöneticime nasıl gönderirim?",
        a: "Kampanyanın Excel raporunu ya da tek sayfalık PDF özetini indirip paylaşabilirsiniz. Excel raporu özet, maliyet, teslimat, alıcılar, segment, etiket ve tahmin ile gerçek karşılaştırmasını ayrı sayfalarda içerir.",
      },
    ],
    related: ["campaigns", "ai", "api"],
  },
  {
    key: "api",
    slug: "api-ve-webhook",
    nav: "API ve webhook",
    icon: "code",
    seoTitle: "WhatsApp API ve Webhook Entegrasyonu",
    seoDescription: "Kendi sisteminizden WhatsApp mesajı gönderin, olayları imzalı webhook'larla alın. Yetki kapsamlı API anahtarları ve firmanıza özel API dokümanıyla entegrasyon.",
    eyebrow: "API ve webhook",
    h1: "WhatsApp API entegrasyonu: kendi sisteminizden mesaj gönderin, olayları anında alın",
    lead: "Sipariş, randevu ya da CRM sisteminiz Messora'nın REST API'siyle WhatsApp mesajı gönderir, kişileri ve kampanyaları yönetir. İmzalı webhook'lar yeni mesajı, teslim durumunu ve abonelikten çıkmayı sisteminize bildirir. Her firmanın API dokümanı kendi adresinde, kendi temel adresiyle yayınlanır.",
    summary: "REST API ile mesaj, kişi ve kampanya yönetin; imzalı webhook'larla olayları sisteminize alın.",
    highlights: ["Yetki kapsamlı API anahtarları", "HMAC-SHA256 imzalı webhook", "Firmaya özel API dokümanı"],
    blocks: [
      {
        title: "REST API",
        body: "Mesaj ve medya gönderin, konuşmaları okuyup yanıtlayın, kişileri ve izinlerini güncelleyin, segmentleri okuyun, kampanya oluşturup başlatın, duraklatın ve sonuçlarını alın. Kampanya ön kontrolü ve ön analiz de API'de vardır. Doküman OpenAPI 3.1 biçimindedir.",
      },
      {
        title: "Güvenli anahtarlar",
        body: "Her API anahtarına yalnızca ihtiyaç duyduğu yetkiyi verin: örneğin yalnızca mesaj gönderme ya da yalnızca kişi okuma. Anahtar başına dakikada 300 istek sınırı uygulanır.",
        points: [
          "Idempotency-Key ile tekrarlanan istek iki kez işlenmez",
          "Tutarlı hata biçimi ve sayfalama",
          "Anahtar oluşturma ve silme denetim kaydına yazılır",
        ],
      },
      {
        title: "Webhook olayları",
        body: "İstediğiniz olayları seçin, Messora her biri için HTTPS adresinize imzalı bir POST isteği gönderir. Olay, değişiklikle aynı veritabanı işleminde kaydedilir: geri alınan bir değişiklik için olay gitmez, gerçekleşen her değişiklik olayını üretir.",
        points: [
          "message.received, message.status",
          "conversation.assigned, conversation.closed, conversation.reopened",
          "campaign.completed, campaign.failed, campaign.cancelled",
          "contact.created, contact.updated, contact.opted_out, contact.opted_in",
        ],
      },
      {
        title: "Teslim edilene kadar dener",
        body: "Sunucunuz yanıt vermezse Messora yaklaşık 24 saat boyunca 10 kez yeniden dener. Teslimat kaydında her denemenin durumu, HTTP kodu ve süresi görünür; bir teslimatı tek tıkla yeniden gönderirsiniz. Olaylar ve teslimat kaydı 30 gün saklanır.",
      },
      {
        title: "İmza ve ağ koruması",
        body: "Her istek, uç noktanıza özel gizli anahtarla HMAC-SHA256 imzası ve zaman damgası taşır; isteğin Messora'dan geldiğini ve değiştirilmediğini doğrularsınız. Webhook adresleri özel ve yerel ağ adreslerine yönlendirilemez; kontrol kayıtta ve her bağlantıda tekrarlanır.",
      },
      {
        title: "Firmanıza özel API dokümanı",
        body: "Her firma kendi adresinde çalışır ve API dokümanı da orada, kendi temel adresiyle yayınlanır. Etkileşimli referansta her işlem için istek ve yanıt örnekleri ile kod örnekleri bulunur.",
      },
    ],
    steps: [
      { title: "Anahtar oluşturun", body: "Panelden yalnızca gereken yetkilerle bir API anahtarı oluşturun." },
      { title: "Dokümanı açın", body: "Firmanıza özel API dokümanında istek örneklerini kendi adresinizle deneyin." },
      { title: "Webhook ekleyin", body: "HTTPS adresinizi kaydedin, olayları seçin, test olayıyla alıcınızı sınayın." },
      { title: "İmzayı doğrulayın", body: "Gelen her isteğin imzasını ve zaman damgasını kontrol edin, olay kimliğiyle tekrarları ayıklayın." },
    ],
    plans: "Başlangıç'ta yalnızca webhook; Büyüme 100 bin, Pro 1 milyon çağrı/ay. Kurumsal'da ihtiyaca göre belirlenir. Her planda anahtar başına dakikada 300 istek sınırı geçerlidir.",
    faq: [
      {
        q: "API ile gönderilen mesajlarda izin kuralları geçerli mi?",
        a: "Evet. Pazarlama iznini kapatmış ya da WhatsApp'ta işletmenizi durdurmuş kişiye pazarlama şablonu API ile de gönderilemez; istek nedenini belirten bir hata koduyla reddedilir. Bu engeli hiçbir ayar ya da parametre kaldıramaz.",
      },
      {
        q: "Aynı isteği yanlışlıkla iki kez gönderirsem?",
        a: "İsteğe bir Idempotency-Key başlığı ekleyin. Aynı anahtarla gelen ikinci istek yeniden işlenmez, ilk isteğin sonucu döner; müşteriye aynı mesaj iki kez gitmez.",
      },
      {
        q: "Sunucum kapalıyken gelen olaylar kaybolur mu?",
        a: "Hayır. Messora başarısız teslimatları yaklaşık 24 saat boyunca 10 kez yeniden dener. Bu süre de dolarsa teslimat kaydından olayı yeniden gönderebilirsiniz. Teslimat en az bir kez yapılır; tekrarları olay kimliğiyle ayıklayın.",
      },
      {
        q: "Meta'nın Cloud API'sini doğrudan kullanmak yerine neden Messora API'si?",
        a: "Meta'nın API'si mesajı taşır. İzin kontrolü, kişi ve kampanya yönetimi, ekip gelen kutusu, maliyet hesabı ve imzalı olaylar Messora'da hazır gelir; aynı kuralları kendiniz yazmanız gerekmez.",
      },
      {
        q: "Başlangıç planında API kullanabilir miyim?",
        a: "Başlangıç planında webhook'larla olayları sisteminize alabilirsiniz; API çağrıları Büyüme planıyla açılır. Büyüme ayda 100 bin, Pro 1 milyon çağrı içerir.",
      },
    ],
    related: ["automations", "campaigns", "inbox"],
  },
];
