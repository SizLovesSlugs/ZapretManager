# Zapret Manager by Siz (beta)

Ускорь Dead by Daylight, YouTube, Discord, Telegram, Instagram, нейросети и прочее за пару кликов!
Без лишних туннелей и бесплатно.

Как юзать? [Скачай](https://github.com/SizLovesSlugs/ZapretManager/releases/latest), запусти, включи.


У разных провайдеров разные стратегии, поэтому протестируйте несколько и остановитесь на той, с которой стабильно работают нужные сервисы.
Проект на стадии тестирования.

**Вопросы или проблемы? Пиши в [Telegram](https://t.me/SizLovesSlugs)**

---

## Что умеет

- Автоматом скачивает и ставит сборки zapret с GitHub (актуальная или выбранная вручную версия)
- Переключает основные и игровые стратегии
- Ускорение Telegram Web, игр и сервисов через hosts
- DNS: Cloudflare, Google, Yandex или системные — применяются при включении службы и откатываются при выключении
- GeoHide‑прокси для работы нейросетей
- Перед первым запуском убирает другие zapret / WinDivert / GoodbyeDPI, чтобы не было конфликта
- Красивый, приятный, понятный интерфейс
- Сам проверяет релизы на GitHub и тихо обновляется, когда выходит новая версия

После **Включения** приложение можно закрыть — служба продолжит работать в фоне.

## Требования

- Windows 10 / 11 x64
- Права администратора (служба, hosts, DNS)
- [Microsoft Edge WebView2 Runtime](https://developer.microsoft.com/microsoft-edge/webview2/) (обычно уже есть)

## Аудит и сборка (для тестирования и проверки)

Будьте бдительны и проверяйте чужие проекты, мой не исключение.

Ничего ставить заранее не нужно.

1. Скачайте исходники:
   ```bat
   git clone https://github.com/SizLovesSlugs/ZapretManager.git
   cd ZapretManager
   ```
   или **Code → Download ZIP** и распакуйте.
2. Запустите **`build.bat`**.

Скрипт скачает portable **Go 1.25** в папку `.tools` (в git не попадает) и соберёт рядом бинарник:

`Zapret Manager x.x.exe`

Если Go уже установлен, можно так:

```bat
go build -trimpath -ldflags "-H windowsgui -s -w" -o "Zapret Manager 1.0 Beta.exe" ./cmd/zapret-manager
```

## Запуск

Запускайте exe **от имени администратора**.  
Антивирус иногда ругается на `winws` / WinDivert — это ожидаемо, не все любят модификацию сетевого трафика.

## Благодарности

Большое спасибо [bol-van](https://github.com/bol-van), [Flowseal](https://github.com/flowseal) и [StressOzz](https://github.com/StressOzz) за Zapret, стратегии и огромный вклад в сообщество!

Проект не связан с авторами zapret официально: это отдельный менеджер поверх их сборок.

## Лицензия

Исходники проекта — как есть.
Zapret, WinDivert и стратегии принадлежат их авторам и распространяются на своих условиях.
