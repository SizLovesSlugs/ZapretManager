module zapret-manager

go 1.25.0

require (
	github.com/jchv/go-webview2 v0.0.0-20260205173254-56598839c808
	github.com/josephspurrier/goversioninfo v1.7.0
	github.com/srwiley/oksvg v0.0.0-20221011165216-be6e8873101c
	github.com/srwiley/rasterx v0.0.0-20220730225603-2ab79fcdd4ef
	golang.org/x/sys v0.47.0
)

// Local patch: hidden create + dark host/WebView background (no white startup flash).
replace github.com/jchv/go-webview2 => ./third_party/go-webview2

require (
	github.com/akavel/rsrc v0.10.2 // indirect
	github.com/jchv/go-winloader v0.0.0-20250406163304-c1995be93bd1 // indirect
	golang.org/x/image v0.0.0-20211028202545-6944b10bf410 // indirect
	golang.org/x/net v0.0.0-20211118161319-6a13c67c3ce4 // indirect
	golang.org/x/text v0.3.6 // indirect
)
