package download

import "testing"

func TestBuildContentDispositionRFC5987(t *testing.T) {
	got := BuildContentDisposition("中文😀.yaml", "subscription.yaml")
	want := `attachment; filename="subscription.yaml"; filename*=UTF-8''%E4%B8%AD%E6%96%87%F0%9F%98%80.yaml`
	if got != want {
		t.Fatalf("Content-Disposition 异常:\nwant %s\n got %s", want, got)
	}
}
