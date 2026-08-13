<div align="center">

# Zapret Manager by Siz (beta)

![Platform](https://img.shields.io/badge/Platform-Windows-0078D6?logo=windows&logoColor=white)
![Architecture](https://img.shields.io/badge/Architecture-X86-6E7681)
![Status](https://img.shields.io/badge/Status-Active-brightgreen)
[![GitHub last commit](https://img.shields.io/github/last-commit/SizLovesSlugs/ZapretManager)](https://github.com/SizLovesSlugs/ZapretManager/commits/main)

Ускорь Dead by Daylight, YouTube, Discord, Telegram, Instagram, нейросети и прочее за пару кликов!  
Без лишних туннелей и бесплатно.

**Как юзать? [Скачай](https://github.com/SizLovesSlugs/ZapretManager/releases/latest), запусти, включи.**

</div>

<img width="1109" height="769" alt="image" src="https://github.com/user-attachments/assets/5ffd84ea-5366-4a0b-8bd2-2359dd3f1b33" />

У разных провайдеров разные стратегии, поэтому протестируйте несколько и остановитесь на той, с которой стабильно работают нужные сервисы.  
Проект на стадии тестирования.

**Вопросы, проблемы, идеи? Пиши в [Telegram](https://t.me/SizLovesSlugs)**

---

## ✨ Что умеет

- Автоматом скачивает и ставит сборки zapret с GitHub (актуальная или выбранная вручную версия)
- Переключает основные и игровые стратегии
- Ускорение Telegram Web, игр и сервисов через hosts
- DNS: Cloudflare, Google, Yandex или системные — применяются при включении службы и откатываются при выключении
- GeoHide‑прокси для работы нейросетей
- Перед первым запуском убирает другие zapret / WinDivert / GoodbyeDPI, чтобы не было конфликта
- Красивый, приятный, понятный интерфейс
- Сам проверяет релизы на GitHub и тихо обновляется, когда выходит новая версия

После **Включения** приложение можно закрыть — служба продолжит работать в фоне.

## 🚀 Запуск

Запускайте exe **от имени администратора**.

> [!WARNING]
> Антивирус иногда ругается на `winws` / WinDivert — это ожидаемо, не все любят модификацию сетевого трафика.

## 📋 Требования

- Windows 10 / 11 x64
- Права администратора (служба, hosts, DNS)
- [Microsoft Edge WebView2 Runtime](https://developer.microsoft.com/microsoft-edge/webview2/) (обычно уже есть)

## 🔍 Аудит и сборка (для тестирования и проверки)

> [!IMPORTANT]
> Будьте бдительны и проверяйте чужие проекты, мой не исключение.

Ничего ставить заранее не нужно.

1. Скачайте исходники:
   ```bat
   git clone https://github.com/SizLovesSlugs/ZapretManager.git
   cd ZapretManager
   ```
   или **Code → Download ZIP** и распакуйте.
2. Запустите **`build.bat`**.

Скрипт скачает portable **Go 1.25** в папку `.tools` (в git не попадает) и соберёт рядом бинарник:

`ZapretManager-x.x.exe`

Если Go уже установлен, можно так:

```bat
go build -trimpath -ldflags "-H windowsgui -s -w" -o ZapretManager-1.0.exe ./cmd/zapret-manager
```

## 🩷 Благодарности

Большое спасибо [bol-van](https://github.com/bol-van), [Flowseal](https://github.com/flowseal) и [StressOzz](https://github.com/StressOzz) за Zapret, стратегии и огромный вклад в сообщество!  
Спасибо Cursor и Grok 4.6 за возможность в свободное время быстро реализовывать классные некомерческие проекты.

Проект не связан с авторами zapret официально: это отдельный менеджер поверх их сборок.

## ⚖️ Лицензия

Исходники проекта — как есть.  
Zapret, WinDivert и стратегии принадлежат их авторам и распространяются на своих условиях.
