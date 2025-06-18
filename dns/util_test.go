package dns

import (
	"testing"
)

func TestDecodeDomain(t *testing.T) {
	domain := "baidu.com"
	data := EncodeDomain(nil, domain)
	if string(DecodeDomain(data)) != domain {
		t.Fail()
	}
}
