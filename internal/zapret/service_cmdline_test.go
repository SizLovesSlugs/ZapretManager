package zapret

import "testing"

func TestSameCmdLine(t *testing.T) {
	exe := `C:\Temp\Zapret Manager\zapret\bin\winws.exe`
	args := []string{"--wf-tcp=443", `--hostlist=C:\Temp\lists\list-general.txt`}
	a := serviceCmdLine(exe, args)
	b := `C:\Temp\Zapret Manager\zapret\bin\winws.exe --wf-tcp=443 --hostlist=C:\Temp\lists\list-general.txt`
	if !sameCmdLine(a, b) {
		t.Fatalf("quoted vs plain:\n%s\n%s", a, b)
	}
	if sameCmdLine(a, serviceCmdLine(exe, []string{"--wf-tcp=80"})) {
		t.Fatal("different args should not match")
	}
}
