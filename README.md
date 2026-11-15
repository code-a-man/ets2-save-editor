# ETS2 Save Editor

Euro Truck Simulator 2 için küçük bir macOS kayıt editörü. Bakiyeyi, seviyeyi, XP’yi, sürüş becerilerini ve ADR sınıflarını düzenler. Aktif tır ve dorseyi onarır, depoyu doldurur. Uygun bir iş öncesi kayıt varsa çevrimdışı Hızlı İş’i cezasız iptal eder.

## İndir ve aç

1. [ETS2 Save Editor’yi indir](https://github.com/code-a-man/ets2-save-editor/releases/latest/download/ETS2-Save-Editor-macOS-universal.zip), ZIP’i aç ve **ETS2 Save Editor.app**’e çift tıkla. macOS 13 ve sonrası için Apple Silicon ve Intel paketi.
2. ETS2’yi kapat. Editör en yeni okunabilir kaydı açar. Başka bir kayıt için **Bulunan Kayıtlar** veya **Seç…** düğmesini kullan.
3. Değerleri düzenle, **Kaydet**’e bas ve ETS2’de düzenlediğin kaydı yükle.

Uygulama noter onaylı olmadığı için macOS ilk açılışı engellerse bir kez açmayı dene. Ardından **Sistem Ayarları → Gizlilik ve Güvenlik → Yine de Aç** seçeneğini kullan.

## Kayıt nerede?

Editör bu macOS konumlarındaki yerel ve Steam Cloud kayıtlarını listeler:

| Konum | Klasör |
| --- | --- |
| Steam Cloud’un yerel kopyası | `~/Library/Application Support/Steam/userdata/<Steam ID>/227300/remote/profiles/` |
| Yerel profiller | `~/Library/Application Support/Euro Truck Simulator 2/` ve `~/Documents/Euro Truck Simulator 2/` altındaki `profiles/` ve `steam_profiles/` |

Özel konumlar için **Seç…** ile `game.sii` dosyasını veya bulunduğu kayıt klasörünü aç. `info.sii` aynı klasörde olmalı. Finder’da gizli bir yola gitmek için `⌘⇧G` kullanabilirsin.

Değişiklikten önce iki dosyanın ZIP yedeği `~/Library/Application Support/ETS2 Save Editor/Yedekler/` klasörüne alınır. Geri dönmek için oyun kapalıyken ZIP’teki `game.sii` ve `info.sii` dosyalarını düzenlediğin kayıt klasörüne kopyala. Steam Cloud çakışma sorarsa hangi kopyayı istediğini tarihine bakarak seç. 1.0.0 yedekleri eski `ETS2 Mini/Yedekler/` klasöründe durur.

## Garajlar

**Garajlar…** seçilen kayıttaki şehirleri ve garaj durumlarını listeler. İstediğin şehirleri veya alınabilecek tüm garajları seçip ana ekranda **Kaydet**’e bas. Yeni garajlar beş boş yerle açılır ve bakiyeden ücret düşmez.

## Hızlı yolculuk ve beceriler

**Hızlı yolculuk için şehirleri aç**, kayıtta bulunan ziyaret edilmemiş şehirleri ziyaret edilmiş olarak işaretler. Hızlı yolculuk noktalarını oyun belirler. Yolların keşif yüzdesi değişmez.

Becerilerin yanındaki **Maks** düğmesi o beceriyi 6. sıraya çıkarır veya tüm ADR sınıflarını açar. Değişiklikler **Kaydet**’e basınca uygulanır.

## Seviye ve Steam ayarı

**Seviye** girersen oyunun varsayılan XP tablosuna göre o seviyenin başlangıç XP’si yazılır. XP tablosunu değiştiren modlar kullanıyorsan **XP** alanını seç.

**Steam Ayarları… → Vulkan + SDR**, ETS2’nin Steam başlatma seçeneklerindeki `-color_mode sdr -rdevice vk` çiftini açar veya kapatır. Ayarı değiştirmek için Steam’den tamamen çık ve **Yenile**’ye bas. Değişiklik bir sonraki oyun açılışında geçerli olur. Vulkan açıkken %100 ölçekleme siyah ekrana neden olabilir. Oyunda %100 dışında bir ölçek seç.

## Güncelle

1.3.0’ı bir kez indirip kur. Sonraki sürümler için **ETS2 Save Editor → Güncellemeleri Denetle…** menüsünü kullan. Sparkle, güncelleme listesinin ve ZIP’in imzasını doğrular. İlk kurulum için macOS’un **Yine de Aç** adımı geçerli olabilir.

## Sürüm yayımla

`v1.3.0` biçiminde bir Git etiketi gönderildiğinde GitHub Actions testleri çalıştırır, Apple Silicon ve Intel için tek macOS ZIP’i derler, `SPARKLE_SIGNING_KEY` sırrıyla ZIP ve güncelleme listesini imzalar ve GitHub Releases’e ekler. **Actions → macOS → Run workflow** ile sürüm oluşturmadan deneme paketi alabilirsin.

## Kaynaktan derle

macOS, Xcode Command Line Tools ve Go 1.24+ gerekir.

```sh
./scripts/build.sh
open "dist/ETS2 Save Editor.app"
```

İki işlemci mimarisi için tek paket almak istersen `./scripts/build.sh universal` çalıştır. Testler için `go test ./...` kullan.

Kayıt çözücü [ets2-decrypt](https://github.com/avoinescu/ets2-decrypt/tree/22c700f70327bd715c358d599ac914187486f48f) projesinden. [MIT lisansı](LICENSE).
