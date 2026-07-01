package api

import (
	"net/http/httptest"
	"testing"
)

func TestParseIdipPageQuery_DefaultsAndCap(t *testing.T) {
	r := httptest.NewRequest("GET", "/?page=2&pageSize=200", nil)
	page, size := parseIdipPageQuery(r)
	if page != 2 || size != 100 {
		t.Fatalf("want page=2 size=100 got %d %d", page, size)
	}
}

func TestParseYyyymmQuery_Invalid(t *testing.T) {
	r := httptest.NewRequest("GET", "/?yyyymm=bad", nil)
	_, ok := parseYyyymmQuery(r, "Asia/Shanghai")
	if ok {
		t.Fatal("expected invalid yyyymm")
	}
}
