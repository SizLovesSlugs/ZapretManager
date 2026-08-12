//go:build windows

package winutil

import "golang.org/x/sys/windows"

var dnsFlush = windows.NewLazySystemDLL("dnsapi.dll").NewProc("DnsFlushResolverCache")

func FlushResolverCache() {
	_, _, _ = dnsFlush.Call()
}
