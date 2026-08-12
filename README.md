# Zapret Manager

Тёмный Windows‑менеджер для [Flowseal zapret](https://github.com/Flowseal/zapret-discord-youtube).  
Одна кнопка — служба, стратегии, DNS и ускорение сервисов.

**Made by [Siz Loves Slugs](https://github.com/SizLovesSlugs) · [Telegram](https://t.me/SizLovesSlugs)**

---

## Что умеет

- Скачивает и ставит сборки zapret с GitHub (актуальная или закреплённая версия)
- Включает / выключает службу `zapret` без лишнего пересоздания
- Переключает основные и игровые стратегии
- Game Filter (UDP / TCP / все)
- Ускорение Telegram Web, игр и сервисов через hosts
- DNS: Cloudflare, Google, Yandex или системные — применяются при включении службы и откатываются при выключении
- GeoHide‑прокси для части сервисов
- Перед первым запуском убирает чужие zapret / WinDivert / GoodbyeDPI, чтобы не было конфликта
- Тёмный интерфейс на WebView2, без белой вспышки при старте

После **Включить** окно можно закрыть — служба продолжит работать в фоне.

## Требования

- Windows 10 / 11 x64
- Права администратора (служба, hosts, DNS)
- [Microsoft Edge WebView2 Runtime](https://developer.microsoft.com/microsoft-edge/webview2/) (обычно уже есть)

## Сборка

Ничего ставить заранее не нужно.

1. Скачайте исходники:
   ```bat
   git clone https://github.com/SizLovesSlugs/ZapretManager.git
   cd ZapretManager
   ```
   или **Code → Download ZIP** и распакуйте.
2. Запустите **`build.bat`**.

Скрипт скачает portable **Go 1.25** в папку `.tools` (в git не попадает) и соберёт рядом:

`Zapret Manager 0.1.exe`

Повторный запуск батника Go заново не качает.

Если Go уже установлен, можно так:

```bat
go build -trimpath -ldflags "-H windowsgui -s -w" -o "Zapret Manager 0.1.exe" ./cmd/zapret-manager
```

## Запуск

Запускайте exe **от имени администратора**.  
Антивирус иногда ругается на `winws` / WinDivert — это ожидаемо для zapret.

## Благодарности

Огромное спасибо [Flowseal](https://github.com/flowseal) и [StressOzz](https://github.com/StressOzz) за стратегии и вклад в сообщество.

Проект не связан с авторами zapret официально: это отдельный менеджер поверх их сборок.

## Лицензия

Исходники этого менеджера — как есть, без гарантий.  
Zapret, WinDivert и стратегии принадлежат их авторам и распространяются на своих условиях.
