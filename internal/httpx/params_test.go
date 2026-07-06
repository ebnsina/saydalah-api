package httpx

import (
	"net/http/httptest"
	"testing"
)

func TestParsePaginationClamps(t *testing.T) {
	cases := []struct {
		query      string
		wantLimit  int32
		wantOffset int32
	}{
		{"", 20, 0},
		{"?page=1&page_size=20", 20, 0},
		{"?page=3&page_size=10", 10, 20},
		{"?page=0", 20, 0},         // page clamps up to 1
		{"?page_size=0", 20, 0},    // invalid size falls back to 20
		{"?page_size=500", 100, 0}, // size caps at 100
	}
	for _, c := range cases {
		r := httptest.NewRequest("GET", "/x"+c.query, nil)
		p := ParsePagination(r)
		if p.Limit != c.wantLimit || p.Offset != c.wantOffset {
			t.Errorf("%q: got limit=%d offset=%d, want limit=%d offset=%d",
				c.query, p.Limit, p.Offset, c.wantLimit, c.wantOffset)
		}
	}
}
