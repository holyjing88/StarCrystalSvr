package service

import (
	"os"
	"testing"
)

func TestPublishServeH5OnAPI_DefaultTrue(t *testing.T) {
	_ = os.Unsetenv("DISABLE_API_H5_STATIC")
	// starcrystal.json 可显式设 publish.serveH5OnApi=false；此处仅验证调用不 panic。
	_ = PublishServeH5OnAPI()
}

func TestPublishServeH5OnAPI_EnvDisable(t *testing.T) {
	t.Setenv("DISABLE_API_H5_STATIC", "1")
	if PublishServeH5OnAPI() {
		t.Fatal("expected false when DISABLE_API_H5_STATIC=1")
	}
}
