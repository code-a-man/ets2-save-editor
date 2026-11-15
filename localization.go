package main

import (
	"fmt"
	"os"
)

func translated(message string) string {
	if os.Getenv("ETS2_LANGUAGE") == "tr" {
		if text, ok := turkishMessages[message]; ok {
			return text
		}
	}
	return message
}

func appError(message string, args ...any) error {
	return fmt.Errorf(translated(message), args...)
}

var turkishMessages = map[string]string{
	"Active job": "Aktif iş",
	"Another save unit references this job’s truck or trailer.": "İşin tırı veya dorsesi başka bir kayıtta kullanılıyor.",
	"Autosave": "Otomatik kayıt",
	"Choose a folder containing game.sii and info.sii.":                            "game.sii ile info.sii dosyalarının bulunduğu klasörü seç.",
	"Choose a regular ETS2 save file.":                                             "Normal bir ETS2 kayıt dosyası seç.",
	"City visit counts do not match.":                                              "Şehir ziyaret sayıları uyuşmuyor.",
	"Close ETS2 before saving.":                                                    "Kaydetmeden önce ETS2’yi kapat.",
	"Could not check whether ETS2 is running.":                                     "Oyunun açık olup olmadığı kontrol edilemedi.",
	"Could not edit %s.":                                                           "%s alanı değiştirilemedi.",
	"Could not find %s in the save from before the job.":                           "İşsiz kayıtta %s bulunamadı.",
	"Could not find ETS2 in Steam settings. Launch it from Steam once.":            "Steam’de ETS2 ayarı bulunamadı. Oyunu Steam’den bir kez aç.",
	"Could not find settings for the last Steam account.":                          "Son Steam hesabının yerel ayarları bulunamadı.",
	"Could not find the %s unit.":                                                  "%s alanı bulunamadı.",
	"Could not identify the Steam account. Sign in to the account that owns ETS2.": "Steam hesabı belirlenemedi. Steam’de ETS2 hesabına giriş yap.",
	"Could not open the detected saves. Use Choose… to open another save.":         "Otomatik bulunan kayıtlar açılamadı. Seç… ile başka bir kayıt aç.",
	"Could not purchase garage %s.":                                                "Bu garaj satın alınamadı: %s",
	"Could not read %s.":                                                           "%s okunamadı.",
	"Could not read Steam settings.":                                               "Steam ayar dosyası okunamadı.",
	"Could not read XP.":                                                           "XP okunamadı.",
	"Could not read the %s skill.":                                                 "%s becerisi okunamadı.",
	"Could not read the Steam launch options.":                                     "Steam başlatma seçenekleri okunamadı.",
	"Could not read the balance.":                                                  "Bakiye okunamadı.",
	"Could not read the save.":                                                     "Kayıt okunamadı.",
	"Could not restore the save. Backup: %s":                                       "Kayıt geri alınamadı. Yedek: %s",
	"Could not verify %s.":                                                         "%s doğrulanamadı.",
	"Could not verify the backup.":                                                 "Yedek doğrulanamadı.",
	"Could not verify which vehicles belong to the job.":                           "İşin araç bağlantıları doğrulanamadı.",
	"Enter a whole number for the balance.":                                        "Bakiye için tam sayı gir.",
	"Enter a whole number of zero or more for level or XP.":                        "Seviye veya XP için pozitif bir tam sayı ya da 0 gir.",
	"Invalid Steam setting.":                                                       "Geçersiz Steam ayarı.",
	"Invalid command.":                                                             "Geçersiz komut.",
	"Invalid job option.":                                                          "Geçersiz iş seçimi.",
	"Invalid skill: %s":                                                            "Geçersiz beceri: %s",
	"Level or XP is out of range.":                                                 "Seviye veya XP desteklenen aralığın dışında.",
	"Local":                                                                        "Yerel",
	"Missing save settings.":                                                       "Eksik kayıt ayarı.",
	"No ETS2 saves found. Use Choose… to open a save folder.":                      "ETS2 kaydı bulunamadı. Seç… ile kayıt klasörünü aç.",
	"No editable truck was found in this save.":                                    "Bu kayıtta düzenlenebilir bir tır bulunamadı.",
	"No save selected.":                                                            "Kayıt seçilmedi.",
	"No suitable save from before this Quick Job was found.":                       "Bu Hızlı İş için uygun bir iş öncesi kayıt bulunamadı.",
	"Put info.sii in the same folder as game.sii.":                                 "info.sii, game.sii ile aynı klasörde olmalı.",
	"Quit Steam completely before changing this setting.":                          "Bu ayarı kaydetmek için Steam’i tamamen kapat.",
	"Remove the conflicting color or renderer option in Steam before enabling this preset.": "Steam’de farklı renk veya render seçeneği var. Önce o seçeneği Steam’den kaldır.",
	"Save vehicle maintenance and job cancellation separately.":                             "Araç bakımı ile iş iptalini ayrı ayrı kaydet.",
	"Set a launch option for ETS2 in Steam first.":                                          "Steam’de önce ETS2 için bir başlatma seçeneği oluştur.",
	"Steam must be fully closed.":                                                           "Steam’in tamamen kapalı olması gerekiyor.",
	"Steam settings changed. Refresh and try again.":                                        "Steam ayarları değişti. Yenile ve tekrar dene.",
	"The save changed. Reload it and try again.":                                            "Kayıt değişmiş. Yenileyip tekrar dene.",
	"The save contains duplicate units.":                                                    "Kayıtta yinelenen birim var.",
	"The save contains more than one %s unit.":                                              "Birden çok %s alanı var.",
	"The save from before the job has a separate GPS route.":                                "İşsiz kayıtta başka GPS rotası var.",
	"There are no changes to save.":                                                         "Kaydedilecek değişiklik yok.",
	"Unexpected job reference: %s":                                                          "Beklenmeyen iş bağlantısı: %s",
	"Unrecognized SII format.":                                                              "SII biçimi tanınmadı.",
	"Choose a save inside an ETS2 profile to edit Driving Academy.":                         "Driving Academy’yi düzenlemek için ETS2 profilinin içindeki bir kaydı seç.",
	"Complete one Driving Academy lesson and close ETS2 first.":                             "Önce Driving Academy’de bir dersi tamamla ve ETS2’yi kapat.",
	"Could not copy the Driving Academy truck.":                                             "Driving Academy tırı kopyalanamadı.",
	"Could not read Driving Academy progress.":                                              "Driving Academy ilerlemesi okunamadı.",
	"Could not verify Driving Academy progress.":                                            "Driving Academy ilerlemesi doğrulanamadı.",
	"This version of the Driving Academy save is unsupported.":                              "Driving Academy kaydının bu sürümü desteklenmiyor.",
}
