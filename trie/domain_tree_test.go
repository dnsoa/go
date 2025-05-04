package trie

import (
	"fmt"
	"testing"

	"github.com/dnsoa/go/assert"
)

func TestDomainTree(t *testing.T) {
	tree := NewDomainTree[int]()
	var np = func(n int) *int {
		return &n
	}

	tests := []struct {
		domain string
		val    *int
	}{
		{
			domain: "example.com",
			val:    np(1),
		},
		{
			domain: "*.c.example.com",
			val:    np(2),
		},
		{
			domain: "www.example.com",
			val:    np(3),
		},
		{
			domain: "sub.c.example.com",
			val:    np(4),
		},
		{
			domain: "*.sub.c.example.com",
			val:    np(5),
		},
		{
			domain: "*.example.com",
			val:    np(6),
		},
		{
			domain: "*.a.a.a.sub.a.example.com",
			val:    np(7),
		},
	}
	for _, test := range tests {
		tree.Add(test.domain, *test.val)
	}
	tests = []struct {
		domain string
		val    *int
	}{
		{
			domain: "example.com",
			val:    np(1),
		},
		{
			domain: "a.example.com",
			val:    np(6),
		},
		{
			domain: "b.c.example.com",
			val:    np(2),
		},
		{
			domain: "b.b.example.com",
			val:    np(6),
		},
		{
			domain: "www.example.com",
			val:    np(3),
		},
		{
			domain: "sub.a.example.com",
			val:    np(6),
		},
		{
			domain: "sub.af.c.example.com",
			val:    np(2),
		},
		{
			domain: "sub.c.example.com",
			val:    np(4),
		},
		{
			domain: "bb.sub.c.example.com",
			val:    np(5),
		},
		{
			domain: "su.a.a.a.sub.a.example.com",
			val:    np(7),
		},
	}
	for _, test := range tests {
		val, _ := tree.Lookup(test.domain)
		// t.Log("query", test.domain, "=>", val)
		assert.Equal(t, *test.val, val)
	}
	tree.Remove("*.a.a.a.sub.a.example.com")
	val, ok := tree.Lookup("su.a.a.a.sub.a.example.com")
	assert.True(t, ok)
	assert.Equal(t, 6, val)
	tree.Print()
	for d := range tree.Database() {
		fmt.Printf("%s=>%d\n", d.Domain, d.Value)
	}
	tree.Reset()
}

func TestDomainTreeFullCoverage(t *testing.T) {
	tree := NewDomainTree[int]()
	var np = func(n int) *int { return &n }

	// 添加各种类型的域名
	addTests := []struct {
		domain string
		val    *int
	}{
		{"example.com", np(1)},
		{"*.example.com", np(2)},
		{"www.example.com", np(3)},
		{"a.b.example.com", np(4)},
		{"*.b.example.com", np(5)},
		{"*.a.b.example.com", np(6)},
		{"c.a.b.example.com", np(7)},
		{"*.c.a.b.example.com", np(8)},
		{"*.d.c.a.b.example.com", np(9)},
		{"d.c.a.b.example.com", np(10)},
	}
	for _, test := range addTests {
		tree.Add(test.domain, *test.val)
	}

	// 查找各种情况
	lookupTests := []struct {
		domain string
		val    *int
		ok     bool
	}{
		{"example.com", np(1), true},           // 精确
		{"www.example.com", np(3), true},       // 精确
		{"foo.example.com", np(2), true},       // 匹配 *.example.com
		{"bar.b.example.com", np(5), true},     // 匹配 *.b.example.com
		{"a.b.example.com", np(4), true},       // 精确
		{"x.a.b.example.com", np(6), true},     // 匹配 *.a.b.example.com
		{"c.a.b.example.com", np(7), true},     // 精确
		{"y.c.a.b.example.com", np(8), true},   // 匹配 *.c.a.b.example.com
		{"d.c.a.b.example.com", np(10), true},  // 精确
		{"z.d.c.a.b.example.com", np(9), true}, // 匹配 *.d.c.a.b.example.com
		{"notfound.com", nil, false},           // 不存在
		{"b.example.com", np(2), true},         // 匹配 *.example.com
		{"a.example.com", np(2), true},         // 匹配 *.example.com
	}

	for _, test := range lookupTests {
		val, ok := tree.Lookup(test.domain)
		//t.Log("query", test.domain, "=>", val)
		if test.ok {
			assert.True(t, ok)
			assert.Equal(t, *test.val, val)
		} else {
			assert.False(t, ok)
		}
	}

	// 删除部分节点并验证
	delTests := []struct {
		domain      string
		lookup      string
		expectVal   *int
		expectFound bool
	}{
		{"*.a.b.example.com", "x.a.b.example.com", np(5), true}, // 删除 *.a.b.example.com，应该退回 *.b.example.com
		{"a.b.example.com", "a.b.example.com", np(5), true},     // 删除 a.b.example.com，应该退回 *.b.example.com
		{"*.b.example.com", "a.b.example.com", np(2), true},     // 删除 *.b.example.com，应该退回 *.example.com
		{"*.example.com", "a.b.example.com", nil, false},        // 删除 *.example.com，应该找不到
	}

	for _, test := range delTests {
		tree.Remove(test.domain)
		val, ok := tree.Lookup(test.lookup)
		if test.expectFound {
			assert.True(t, ok)
			assert.Equal(t, *test.expectVal, val)
		} else {
			assert.False(t, ok)
		}
	}

	// 删除所有，树应为空
	for _, test := range addTests {
		tree.Remove(test.domain)
	}
	for _, test := range addTests {
		val, ok := tree.Lookup(test.domain)
		assert.False(t, ok)
		assert.Equal(t, 0, val)
	}
	tree.Print()
}

func BenchmarkDomainTreeAdd(b *testing.B) {
	tree := NewDomainTree[int]()

	for i := 0; b.Loop(); i++ {
		tree.Add(fmt.Sprintf("%d.example.com", i), i)
	}
}

func BenchmarkDomainTreeLookup(b *testing.B) {
	tree := NewDomainTree[int]()
	tests := []struct {
		domain string
		val    int
	}{
		{
			domain: "example.com",
			val:    1,
		},
		{
			domain: "*.example.com",
			val:    2,
		},
		{
			domain: "www.example.com",
			val:    3,
		},
		{
			domain: "sub.a.example.com",
			val:    4,
		},
	}
	for _, test := range tests {
		tree.Add(test.domain, test.val)
	}

	for b.Loop() {
		tree.Lookup("3.sub.a.example.com")
	}
}
