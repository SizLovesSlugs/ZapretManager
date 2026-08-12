# Zapret Manager patches to go-webview2

Based on `github.com/jchv/go-webview2` @ `56598839c808`.

1. Window class uses a dark `HbrBackground` (`#0a0b0e`) so the first host paint is never white.
2. On controller create, `PutDefaultBackgroundColor` is set to `#0a0b0e`.
3. `Show()` kept for API completeness (window is shown during create).
