# ETS2 Mini

Euro Truck Simulator 2 için küçük bir macOS kayıt editörü. Bakiyeyi, sürüş becerilerini ve ADR sınıflarını düzenler. Uygun bir iş öncesi kayıt varsa çevrimdışı Hızlı İş’i cezasız iptal eder.

## İndir ve aç

1. [ETS2 Mini’yi indir](https://github.com/code-a-man/ets2-mini/releases/latest/download/ETS2-Mini-macOS-universal.zip), ZIP’i aç ve **ETS2 Mini.app**’e çift tıkla. macOS 13 ve sonrası için Apple Silicon ve Intel paketi.
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

Değişiklikten önce iki dosyanın ZIP yedeği `~/Library/Application Support/ETS2 Mini/Yedekler/` klasörüne alınır. Geri dönmek için oyun kapalıyken ZIP’teki `game.sii` ve `info.sii` dosyalarını düzenlediğin kayıt klasörüne kopyala. Steam Cloud çakışma sorarsa hangi kopyayı istediğini tarihine bakarak seç.

## Kaynaktan derle

macOS, Xcode Command Line Tools ve Go 1.24+ gerekir.

```sh
./scripts/build.sh
open "dist/ETS2 Mini.app"
```

İki işlemci mimarisi için tek paket almak istersen `./scripts/build.sh universal` çalıştır. Testler için `go test ./...` kullan.

Kayıt çözücü [ets2-decrypt](https://github.com/avoinescu/ets2-decrypt/tree/22c700f70327bd715c358d599ac914187486f48f) projesinden. [MIT lisansı](LICENSE).
