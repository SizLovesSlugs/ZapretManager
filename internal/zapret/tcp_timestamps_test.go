package zapret

import "testing"

func TestParseTCPTimestampsEnabled(t *testing.T) {
	en := `
TCP Global Parameters
----------------------------------------------
Receive-Side Scaling State          : enabled
Chimney Offload State               : automatic
Timestamps                          : enabled
`
	if !parseTCPTimestampsEnabled(en) {
		t.Fatal("expected enabled")
	}
	dis := `
Timestamps                          : disabled
`
	if parseTCPTimestampsEnabled(dis) {
		t.Fatal("expected disabled")
	}
	ruOn := `
Метки времени                       : включен
`
	if !parseTCPTimestampsEnabled(ruOn) {
		t.Fatal("expected RU enabled")
	}
	ruOff := `
Метки времени                       : отключен
`
	if parseTCPTimestampsEnabled(ruOff) {
		t.Fatal("expected RU disabled")
	}
	if parseTCPTimestampsEnabled("no timestamps line") {
		t.Fatal("missing line should be treated as not enabled")
	}
}
