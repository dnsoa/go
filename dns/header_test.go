package dns

import (
	"testing"

	"github.com/dnsoa/go/assert"
)

func TestHeader(t *testing.T) {
	r := assert.New(t)
	h := &Header{}
	// r.Equal(RcodeSuccess, h.Rcode())
	// h.SetRcode(RcodeBadName)
	// r.Equal(RcodeBadName, h.Rcode())
	r.False(h.Authoritative())
	h.SetAuthoritative()
	r.True(h.Authoritative())
	r.False(h.Truncated())
	h.SetTruncated()
	r.True(h.Truncated())
	r.False(h.RecursionDesired())
	h.SetRecursionDesired()
	r.True(h.RecursionDesired())
	r.False(h.RecursionAvailable())
	h.SetRecursionAvailable()
	r.True(h.RecursionAvailable())
	r.False(h.Zero())
	h.SetZero()
	r.True(h.Zero())
	r.False(h.AuthenticatedData())
	h.SetAuthenticatedData()
	r.True(h.AuthenticatedData())
	r.False(h.CheckingDisabled())
	h.SetCheckingDisabled()
	r.True(h.CheckingDisabled())
	r.Equal(OpcodeQuery, h.OpCode())
	h.SetOpCode(OpcodeNotify)
	r.Equal(OpcodeNotify, h.OpCode())
	r.False(h.Response())
	h.SetResponse()
	r.True(h.Response())

}
