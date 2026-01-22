package pugixml

import (
	"bytes"
	"encoding/xml"
	"io"
	"testing"
)

// TestSimpleElement 测试简单元素解析
func TestSimpleElement(t *testing.T) {
	input := []byte(`<root/>`)
	doc, err := NewParser(input).Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if doc.Type != NodeDocument {
		t.Errorf("Expected NodeDocument, got %v", doc.Type)
	}

	root := doc.FirstChild
	if root == nil {
		t.Fatal("Expected root element, got nil")
	}

	if root.Type != NodeElement {
		t.Errorf("Expected NodeElement, got %v", root.Type)
	}

	if !bytes.Equal(root.Name, []byte("root")) {
		t.Errorf("Expected name 'root', got %s", root.Name)
	}
}

// TestElementWithAttributes 测试带属性的元素
func TestElementWithAttributes(t *testing.T) {
	input := []byte(`<root id="123" name="test"/>`)
	doc, err := NewParser(input).Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	root := doc.FirstChild
	attrs := root.Attrs()

	if len(attrs) != 2 {
		t.Fatalf("Expected 2 attributes, got %d", len(attrs))
	}

	if !bytes.Equal(attrs[0].Name, []byte("id")) {
		t.Errorf("Expected attr name 'id', got %s", attrs[0].Name)
	}
	if !bytes.Equal(attrs[0].Value, []byte("123")) {
		t.Errorf("Expected attr value '123', got %s", attrs[0].Value)
	}

	if !bytes.Equal(attrs[1].Name, []byte("name")) {
		t.Errorf("Expected attr name 'name', got %s", attrs[1].Name)
	}
	if !bytes.Equal(attrs[1].Value, []byte("test")) {
		t.Errorf("Expected attr value 'test', got %s", attrs[1].Value)
	}
}

// TestNestedElements 测试嵌套元素
func TestNestedElements(t *testing.T) {
	input := []byte(`<root><child><grandchild/></child></root>`)
	doc, err := NewParser(input).Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	root := doc.FirstChild
	if !bytes.Equal(root.Name, []byte("root")) {
		t.Fatalf("Expected root name 'root', got %s", root.Name)
	}

	child := root.FirstChild
	if child == nil {
		t.Fatal("Expected child element, got nil")
	}
	if !bytes.Equal(child.Name, []byte("child")) {
		t.Errorf("Expected child name 'child', got %s", child.Name)
	}

	grandchild := child.FirstChild
	if grandchild == nil {
		t.Fatal("Expected grandchild element, got nil")
	}
	if !bytes.Equal(grandchild.Name, []byte("grandchild")) {
		t.Errorf("Expected grandchild name 'grandchild', got %s", grandchild.Name)
	}
}

// TestTextContent 测试文本内容
func TestTextContent(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []byte
	}{
		{
			name:     "simple text",
			input:    []byte(`<root>Hello World</root>`),
			expected: []byte("Hello World"),
		},
		{
			name:     "text with entities",
			input:    []byte(`<root>Hello &lt;World&gt;</root>`),
			expected: []byte("Hello <World>"),
		},
		{
			name:     "text with ampersand",
			input:    []byte(`<root>Tom &amp; Jerry</root>`),
			expected: []byte("Tom & Jerry"),
		},
		{
			name:     "text with quote",
			input:    []byte(`<root>&quot;quoted&quot;</root>`),
			expected: []byte("\"quoted\""),
		},
		{
			name:     "text with apostrophe",
			input:    []byte(`<root>&apos;apostrophe&apos;</root>`),
			expected: []byte("'apostrophe'"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := NewParser(tt.input).Parse()
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			root := doc.FirstChild
			textNode := root.FirstChild

			if textNode == nil {
				t.Fatal("Expected text node, got nil")
			}
			if textNode.Type != NodePCDATA {
				t.Errorf("Expected NodePCDATA, got %v", textNode.Type)
			}
			if !bytes.Equal(textNode.Value, tt.expected) {
				t.Errorf("Expected text %q, got %q", tt.expected, textNode.Value)
			}
		})
	}
}

// TestNumericEntities 测试数字实体引用
func TestNumericEntities(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []byte
	}{
		{
			name:     "decimal entity",
			input:    []byte(`<root>&#65;</root>`),
			expected: []byte("A"),
		},
		{
			name:     "hex entity lowercase",
			input:    []byte(`<root>&#x41;</root>`),
			expected: []byte("A"),
		},
		{
			name:     "hex entity uppercase",
			input:    []byte(`<root>&#X41;</root>`),
			expected: []byte("A"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := NewParser(tt.input).Parse()
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			root := doc.FirstChild
			textNode := root.FirstChild

			if !bytes.Equal(textNode.Value, tt.expected) {
				t.Errorf("Expected text %q, got %q", tt.expected, textNode.Value)
			}
		})
	}
}

// TestNumericEntitiesUnicode 测试 Unicode 数字实体引用
func TestNumericEntitiesUnicode(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:     "decimal large",
			input:    []byte(`<root>&#128512;</root>`),
			expected: "😀",
		},
		{
			name:     "hex emoji",
			input:    []byte(`<root>&#x1F600;</root>`),
			expected: "😀",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := NewParser(tt.input).Parse()
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			root := doc.FirstChild
			textNode := root.FirstChild

			if string(textNode.Value) != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, string(textNode.Value))
			}
		})
	}
}

func BenchmarkParseLargeDocument(b *testing.B) {
	var buf bytes.Buffer
	buf.WriteString("<root>")
	for i := 0; i < 2000; i++ {
		buf.WriteString("<item>Some text &#x1F600;</item>")
	}
	buf.WriteString("</root>")
	data := buf.Bytes()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := NewParser(data).Parse()
		if err != nil {
			b.Fatalf("Parse failed: %v", err)
		}
	}
}

func BenchmarkCompareStdXML(b *testing.B) {
	var buf bytes.Buffer
	buf.WriteString("<root>")
	for i := 0; i < 2000; i++ {
		buf.WriteString("<item attr=\"v\">Some text &amp; more</item>")
	}
	buf.WriteString("</root>")
	data := buf.Bytes()

	b.Run("pugixml", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := NewParser(data).Parse()
			if err != nil {
				b.Fatalf("pugixml parse failed: %v", err)
			}
		}
	})

	b.Run("encoding/xml", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			dec := xml.NewDecoder(bytes.NewReader(data))
			for {
				_, err := dec.Token()
				if err != nil {
					if err == io.EOF {
						break
					}
					b.Fatalf("encoding/xml token failed: %v", err)
				}
			}
		}
	})
}

// TestComment 测试注释
func TestComment(t *testing.T) {
	input := []byte(`<root><!-- This is a comment --></root>`)
	doc, err := NewParser(input).Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	root := doc.FirstChild
	comment := root.FirstChild

	if comment == nil {
		t.Fatal("Expected comment node, got nil")
	}
	if comment.Type != NodeComment {
		t.Errorf("Expected NodeComment, got %v", comment.Type)
	}
	if !bytes.Equal(comment.Value, []byte(" This is a comment ")) {
		t.Errorf("Expected comment ' This is a comment ', got %q", comment.Value)
	}
}

// TestCDATA 测试 CDATA 段
func TestCDATA(t *testing.T) {
	input := []byte(`<root><![CDATA[<>&<script>alert(1)</script>]]></root>`)
	doc, err := NewParser(input).Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	root := doc.FirstChild
	cdata := root.FirstChild

	if cdata == nil {
		t.Fatal("Expected CDATA node, got nil")
	}
	if cdata.Type != NodeCDATA {
		t.Errorf("Expected NodeCDATA, got %v", cdata.Type)
	}
	expected := []byte("<>&<script>alert(1)</script>")
	if !bytes.Equal(cdata.Value, expected) {
		t.Errorf("Expected CDATA %q, got %q", expected, cdata.Value)
	}
}

// TestProcessingInstruction 测试处理指令
func TestProcessingInstruction(t *testing.T) {
	input := []byte(`<?xml version="1.0" encoding="UTF-8"?><root/>`)
	doc, err := NewParser(input).Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	pi := doc.FirstChild
	if pi == nil {
		t.Fatal("Expected PI node, got nil")
	}
	if pi.Type != NodePI {
		t.Errorf("Expected NodePI, got %v", pi.Type)
	}
	if !bytes.HasPrefix(pi.Value, []byte("xml version")) {
		t.Errorf("Expected PI content starting with 'xml version', got %q", pi.Value)
	}
}

// TestMixedContent 测试混合内容
func TestMixedContent(t *testing.T) {
	input := []byte(`<root>Text1<b/>Text2<c/>Text3</root>`)
	doc, err := NewParser(input).Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	root := doc.FirstChild
	children := root.ChildNodes()

	if len(children) != 5 {
		t.Fatalf("Expected 5 children, got %d", len(children))
	}

	// Text1
	if children[0].Type != NodePCDATA {
		t.Errorf("Expected NodePCDATA at index 0, got %v", children[0].Type)
	}
	if !bytes.Equal(children[0].Value, []byte("Text1")) {
		t.Errorf("Expected 'Text1', got %q", children[0].Value)
	}

	// <b/>
	if children[1].Type != NodeElement {
		t.Errorf("Expected NodeElement at index 1, got %v", children[1].Type)
	}
	if !bytes.Equal(children[1].Name, []byte("b")) {
		t.Errorf("Expected element name 'b', got %q", children[1].Name)
	}

	// Text2
	if !bytes.Equal(children[2].Value, []byte("Text2")) {
		t.Errorf("Expected 'Text2', got %q", children[2].Value)
	}

	// <c/>
	if !bytes.Equal(children[3].Name, []byte("c")) {
		t.Errorf("Expected element name 'c', got %q", children[3].Name)
	}

	// Text3
	if !bytes.Equal(children[4].Value, []byte("Text3")) {
		t.Errorf("Expected 'Text3', got %q", children[4].Value)
	}
}

// TestMultipleRootElements 测试多个根元素
func TestMultipleRootElements(t *testing.T) {
	input := []byte(`<a/><b/><c/>`)
	doc, err := NewParser(input).Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	children := doc.ChildNodes()
	if len(children) != 3 {
		t.Fatalf("Expected 3 root elements, got %d", len(children))
	}

	expectedNames := [][]byte{[]byte("a"), []byte("b"), []byte("c")}
	for i, child := range children {
		if !bytes.Equal(child.Name, expectedNames[i]) {
			t.Errorf("Child %d: expected name %q, got %q", i, expectedNames[i], child.Name)
		}
	}
}

// TestSiblingElements 测试兄弟元素
func TestSiblingElements(t *testing.T) {
	input := []byte(`<root><a/><b/><c/></root>`)
	doc, err := NewParser(input).Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	root := doc.FirstChild
	children := root.ChildNodes()

	if len(children) != 3 {
		t.Fatalf("Expected 3 children, got %d", len(children))
	}

	// 验证 NextSibling 链
	if root.FirstChild != children[0] {
		t.Error("FirstChild mismatch")
	}
	if root.LastChild != children[2] {
		t.Error("LastChild mismatch")
	}

	if children[0].NextSibling != children[1] {
		t.Error("NextSibling chain broken: 0 -> 1")
	}
	if children[1].NextSibling != children[2] {
		t.Error("NextSibling chain broken: 1 -> 2")
	}
	if children[2].NextSibling != nil {
		t.Error("NextSibling chain broken: 2 -> nil")
	}
}

// TestWhitespaceHandling 测试空白处理
func TestWhitespaceHandling(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		wantText []byte
	}{
		{
			name:     "preserve whitespace",
			input:    []byte(`<root>spaced</root>`),
			wantText: []byte("spaced"),
		},
		{
			name:     "newlines preserved",
			input:    []byte("<root>line1\nline2</root>"),
			wantText: []byte("line1\nline2"),
		},
		{
			name:     "tabs preserved",
			input:    []byte("<root>\ttabbed\t</root>"),
			wantText: []byte("tabbed\t"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := NewParser(tt.input).Parse()
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			root := doc.FirstChild
			if root.FirstChild == nil {
				t.Fatal("Expected text node, got nil")
			}
			if !bytes.Equal(root.FirstChild.Value, tt.wantText) {
				t.Errorf("Text = %q, want %q", root.FirstChild.Value, tt.wantText)
			}
		})
	}
}

// TestCarriageReturnNormalization 测试回车符标准化
func TestCarriageReturnNormalization(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []byte
	}{
		{
			name:     "CR to LF",
			input:    []byte("<root>text\rmore</root>"),
			expected: []byte("text\nmore"),
		},
		{
			name:     "CRLF to LF",
			input:    []byte("<root>text\r\nmore</root>"),
			expected: []byte("text\nmore"),
		},
		{
			name:     "CR in middle",
			input:    []byte("<root>start\rmiddle\nend</root>"),
			expected: []byte("start\nmiddle\nend"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := NewParser(tt.input).Parse()
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			root := doc.FirstChild
			textNode := root.FirstChild
			if textNode == nil {
				t.Fatal("Expected text node, got nil")
			}

			if !bytes.Equal(textNode.Value, tt.expected) {
				t.Errorf("Expected %q, got %q", tt.expected, textNode.Value)
			}
		})
	}
}

// TestSingleQuoteAttributes 测试单引号属性
func TestSingleQuoteAttributes(t *testing.T) {
	input := []byte(`<root id='123' name='test'/>`)
	doc, err := NewParser(input).Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	root := doc.FirstChild
	val, ok := root.GetAttr([]byte("id"))
	if !ok {
		t.Fatal("Attribute 'id' not found")
	}
	if !bytes.Equal(val, []byte("123")) {
		t.Errorf("Expected '123', got %q", val)
	}

	val, ok = root.GetAttr([]byte("name"))
	if !ok {
		t.Fatal("Attribute 'name' not found")
	}
	if !bytes.Equal(val, []byte("test")) {
		t.Errorf("Expected 'test', got %q", val)
	}
}

// TestMixedQuoteAttributes 测试混合引号属性
func TestMixedQuoteAttributes(t *testing.T) {
	input := []byte(`<root id="123" name='test' version="1.0"/>`)
	doc, err := NewParser(input).Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	root := doc.FirstChild
	attrs := root.Attrs()

	if len(attrs) != 3 {
		t.Fatalf("Expected 3 attributes, got %d", len(attrs))
	}
}

// TestAttributeWithoutValue 测试无值属性
func TestAttributeWithoutValue(t *testing.T) {
	input := []byte(`<root checked disabled/>`)
	doc, err := NewParser(input).Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	root := doc.FirstChild
	attrs := root.Attrs()

	if len(attrs) != 2 {
		t.Fatalf("Expected 2 attributes, got %d", len(attrs))
	}

	// 无值属性的值应该为空
	if !bytes.Equal(attrs[0].Value, []byte{}) {
		t.Errorf("Expected empty value, got %q", attrs[0].Value)
	}
}

// TestAttributeEntities 测试属性中的实体引用
func TestAttributeEntities(t *testing.T) {
	input := []byte(`<root text="Hello &lt;World&gt;"/>`)
	doc, err := NewParser(input).Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	root := doc.FirstChild
	val, ok := root.GetAttr([]byte("text"))
	if !ok {
		t.Fatal("Attribute 'text' not found")
	}

	expected := []byte("Hello <World>")
	if !bytes.Equal(val, expected) {
		t.Errorf("Expected %q, got %q", expected, val)
	}
}

// TestComplexDocument 测试复杂文档
func TestComplexDocument(t *testing.T) {
	input := []byte(`<?xml version="1.0"?>
<root id="main">
	<!-- This is a comment -->
	<metadata>
		<author>John Doe</author>
		<date>2024-01-01</date>
	</metadata>
	<content>
		<p>Paragraph 1</p>
		<p>Paragraph 2 with &lt;entity&gt;</p>
		<data><![CDATA[special <>& chars]]></data>
	</content>
</root>`)

	doc, err := NewParser(input).Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// 检查 PI
	pi := doc.FirstChild
	if pi.Type != NodePI {
		t.Errorf("Expected PI as first child, got %v", pi.Type)
	}

	// 检查 root
	root := pi.NextSibling
	if !bytes.Equal(root.Name, []byte("root")) {
		t.Errorf("Expected root element, got %s", root.Name)
	}

	// 检查属性
	val, ok := root.GetAttr([]byte("id"))
	if !ok || !bytes.Equal(val, []byte("main")) {
		t.Error("Expected id='main'")
	}

	// 检查 metadata
	metadata := root.FindChildByName([]byte("metadata"))
	if metadata == nil {
		t.Fatal("metadata element not found")
	}

	// 检查 content
	content := root.FindChildByName([]byte("content"))
	if content == nil {
		t.Fatal("content element not found")
	}

	// 检查 content 中的 CDATA
	foundCDATA := false
	for child := content.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == NodeElement && bytes.Equal(child.Name, []byte("data")) {
			data := child
			for dc := data.FirstChild; dc != nil; dc = dc.NextSibling {
				if dc.Type == NodeCDATA {
					foundCDATA = true
					if !bytes.Contains(dc.Value, []byte("special <>& chars")) {
						t.Errorf("CDATA content mismatch: %q", dc.Value)
					}
				}
			}
		}
	}
	if !foundCDATA {
		t.Error("CDATA section not found in content")
	}
}

// TestErrors 测试错误处理
func TestErrors(t *testing.T) {
	tests := []struct {
		name        string
		input       []byte
		expectedErr string
	}{
		{
			name:        "unterminated comment",
			input:       []byte(`<root><!-- unclosed`),
			expectedErr: "unterminated comment",
		},
		{
			name:        "unterminated CDATA",
			input:       []byte(`<root><![CDATA[unclosed`),
			expectedErr: "unterminated CDATA",
		},
		{
			name:        "mismatched closing tag",
			input:       []byte(`<root></other>`),
			expectedErr: "mismatched closing tag",
		},
		{
			name:        "unterminated element",
			input:       []byte(`<root><child></root>`), // mismatched: child not closed before root closes
			expectedErr: "mismatched closing tag",
		},
		{
			name:        "unterminated attribute value",
			input:       []byte(`<root attr="value`),
			expectedErr: "unterminated attribute value",
		},
		{
			name:        "unquoted attribute value",
			input:       []byte(`<root attr=value>`),
			expectedErr: "attribute value must be quoted",
		},
		{
			name:        "unterminated PI",
			input:       []byte(`<?xml version="1.0"`),
			expectedErr: "unterminated processing instruction",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewParser(tt.input).Parse()
			if err == nil {
				t.Fatal("Expected error, got nil")
			}
			if tt.expectedErr != "" && !contains(err.Error(), tt.expectedErr) {
				t.Errorf("Error message should contain %q, got %q", tt.expectedErr, err.Error())
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestEmptyDocument 测试空文档
func TestEmptyDocument(t *testing.T) {
	tests := [][]byte{
		{},
		[]byte(""),
		[]byte("   "),
		[]byte("\n\t\n"),
	}

	for _, input := range tests {
		doc, err := NewParser(input).Parse()
		if err != nil {
			t.Fatalf("Parse failed for %q: %v", input, err)
		}
		if doc.Type != NodeDocument {
			t.Errorf("Expected NodeDocument for empty input, got %v", doc.Type)
		}
		if doc.FirstChild != nil {
			t.Errorf("Expected no children for empty input, got %v", doc.FirstChild)
		}
	}
}

// TestNodeNavigation 测试节点导航
func TestNodeNavigation(t *testing.T) {
	input := []byte(`<root><a><x/></a><b><y/></b><c><z/></c></root>`)
	doc, err := NewParser(input).Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	root := doc.FirstChild

	// 测试 FindChildByName
	a := root.FindChildByName([]byte("a"))
	if a == nil {
		t.Fatal("Element 'a' not found")
	}

	b := root.FindChildByName([]byte("b"))
	if b == nil {
		t.Fatal("Element 'b' not found")
	}

	c := root.FindChildByName([]byte("c"))
	if c == nil {
		t.Fatal("Element 'c' not found")
	}

	// 验证父关系
	if a.Parent != root {
		t.Error("Parent relationship broken")
	}

	// 验证子节点
	x := a.FirstChild
	if !bytes.Equal(x.Name, []byte("x")) {
		t.Error("Child navigation failed")
	}
}

// TestGetAttr 测试 GetAttr 方法
func TestGetAttr(t *testing.T) {
	input := []byte(`<root id="123" name="test" version="1.0"/>`)
	doc, err := NewParser(input).Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	root := doc.FirstChild

	// 测试存在的属性
	val, ok := root.GetAttr([]byte("id"))
	if !ok {
		t.Error("Attribute 'id' not found")
	}
	if !bytes.Equal(val, []byte("123")) {
		t.Errorf("Expected '123', got %q", val)
	}

	val, ok = root.GetAttr([]byte("name"))
	if !ok {
		t.Error("Attribute 'name' not found")
	}
	if !bytes.Equal(val, []byte("test")) {
		t.Errorf("Expected 'test', got %q", val)
	}

	// 测试不存在的属性
	_, ok = root.GetAttr([]byte("nonexistent"))
	if ok {
		t.Error("Nonexistent attribute should not be found")
	}
}

// TestAttributesHelpers 测试 Attributes helper 方法
func TestAttributesHelpers(t *testing.T) {
	input := []byte(`<root id="123" name="test"/>`)
	doc, err := NewParser(input).Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	root := doc.FirstChild
	attrs := root.Attrs()

	if !attrs.Has([]byte("id")) {
		t.Error("attrs.Has(id) should be true")
	}

	if attrs.Has([]byte("missing")) {
		t.Error("attrs.Has(missing) should be false")
	}

	if v, ok := attrs.Get([]byte("name")); !ok || !bytes.Equal(v, []byte("test")) {
		t.Errorf("attrs.Get(name) expected 'test', got %q (ok=%v)", v, ok)
	}

	if a := attrs.Find([]byte("id")); a == nil || !bytes.Equal(a.Value, []byte("123")) {
		t.Errorf("attrs.Find(id) expected attribute with value '123', got %v", a)
	}
}

func TestAttributesMap(t *testing.T) {
	input := []byte(`<root id="1" name="old" flag/>`)
	doc, err := NewParser(input).Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	root := doc.FirstChild
	attrs := root.Attrs()

	// Modify an attribute value using Map
	for a := range attrs.Map() {
		if bytes.Equal(a.Name, []byte("name")) {
			a.Value = []byte("new")
		}
	}

	if v, ok := attrs.Get([]byte("name")); !ok || !bytes.Equal(v, []byte("new")) {
		t.Fatalf("attrs.Map didn't update attribute: got %q (ok=%v)", v, ok)
	}

	// Collect names in order to ensure iteration order is stable
	var names [][]byte
	for a := range attrs.Map() {
		names = append(names, a.Name)
	}

	if len(names) != len(attrs) {
		t.Fatalf("expected %d names, got %d", len(attrs), len(names))
	}

	// Ensure the first attribute is 'id'
	if !bytes.Equal(names[0], []byte("id")) {
		t.Fatalf("expected first attr name 'id', got %q", names[0])
	}
}

// TestEmptyElementName 测试空元素名错误
func TestEmptyElementName(t *testing.T) {
	input := []byte(`<></>`)
	_, err := NewParser(input).Parse()
	if err == nil {
		t.Fatal("Expected error for empty element name")
	}
}

// TestUnterminatedInput 测试未终止的输入
func TestUnterminatedInput(t *testing.T) {
	input := []byte(`<root`)
	_, err := NewParser(input).Parse()
	if err == nil {
		t.Fatal("Expected error for unterminated input")
	}
}

// TestUnicodeInContent 测试 Unicode 内容
func TestUnicodeInContent(t *testing.T) {
	input := []byte(`<root>Hello 世界 🌍</root>`)
	doc, err := NewParser(input).Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	root := doc.FirstChild
	textNode := root.FirstChild

	expected := "Hello 世界 🌍"
	if string(textNode.Value) != expected {
		t.Errorf("Expected %q, got %q", expected, textNode.Value)
	}
}

// TestDeepNesting 测试深层嵌套
func TestDeepNesting(t *testing.T) {
	var buildXML func(depth int) string
	buildXML = func(depth int) string {
		if depth == 0 {
			return "<leaf/>"
		}
		return "<a>" + buildXML(depth-1) + "</a>"
	}

	deepXML := []byte(buildXML(100))
	_, err := NewParser(deepXML).Parse()
	if err != nil {
		t.Fatalf("Parse failed for deeply nested XML: %v", err)
	}
}

// TestManyAttributes 测试多个属性
func TestManyAttributes(t *testing.T) {
	input := []byte(`<root a1="1" a2="2" a3="3" a4="4" a5="5" a6="6" a7="7" a8="8" a9="9" a10="10"/>`)
	doc, err := NewParser(input).Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	root := doc.FirstChild
	attrs := root.Attrs()

	if len(attrs) != 10 {
		t.Fatalf("Expected 10 attributes, got %d", len(attrs))
	}

	expectedNames := [][]byte{[]byte("a1"), []byte("a2"), []byte("a3"), []byte("a4"), []byte("a5"),
		[]byte("a6"), []byte("a7"), []byte("a8"), []byte("a9"), []byte("a10")}
	for i := 0; i < 10; i++ {
		if !bytes.Equal(attrs[i].Name, expectedNames[i]) {
			t.Errorf("Attr %d: expected name %q, got %q", i, expectedNames[i], attrs[i].Name)
		}
	}
}

// TestManySiblings 测试多个兄弟节点
func TestManySiblings(t *testing.T) {
	buildXML := func(count int) string {
		result := "<root>"
		for i := 0; i < count; i++ {
			result += "<item/>"
		}
		result += "</root>"
		return result
	}

	input := []byte(buildXML(100))
	doc, err := NewParser(input).Parse()
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	root := doc.FirstChild
	count := 0
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		count++
	}

	if count != 100 {
		t.Errorf("Expected 100 siblings, got %d", count)
	}
}

// Benchmark tests

func BenchmarkParserSimpleElement(b *testing.B) {
	input := []byte(`<root/>`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NewParser(input).Parse()
	}
}

func BenchmarkParserWithAttributes(b *testing.B) {
	input := []byte(`<root id="123" name="test" version="1.0"/>`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NewParser(input).Parse()
	}
}

func BenchmarkParserNestedElements(b *testing.B) {
	input := []byte(`<root><a><b><c><d><e/></d></c></b></a></root>`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NewParser(input).Parse()
	}
}

func BenchmarkParserTextContent(b *testing.B) {
	input := []byte(`<root>Hello World, this is a test with some text content.</root>`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NewParser(input).Parse()
	}
}

func BenchmarkParserWithEntities(b *testing.B) {
	input := []byte(`<root>&lt;tag&gt; &amp; &quot;quotes&quot; &apos;apost&apos;</root>`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NewParser(input).Parse()
	}
}

func BenchmarkParserMixedContent(b *testing.B) {
	input := []byte(`<root>Text1<b/>Text2<c/>Text3<d/>Text4</root>`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NewParser(input).Parse()
	}
}

func BenchmarkParserManyAttributes(b *testing.B) {
	input := []byte(`<root a1="1" a2="2" a3="3" a4="4" a5="5" a6="6" a7="7" a8="8" a9="9" a10="10"/>`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NewParser(input).Parse()
	}
}

func BenchmarkParserComplexDocument(b *testing.B) {
	input := []byte(`<?xml version="1.0"?>
<root id="main">
	<!-- This is a comment -->
	<metadata>
		<author>John Doe</author>
		<date>2024-01-01</date>
	</metadata>
	<content>
		<p>Paragraph 1</p>
		<p>Paragraph 2 with &lt;entity&gt;</p>
		<data><![CDATA[special <>& chars]]></data>
	</content>
</root>`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NewParser(input).Parse()
	}
}

func BenchmarkParserManySiblings(b *testing.B) {
	buildXML := func(count int) string {
		result := "<root>"
		for i := 0; i < count; i++ {
			result += "<item/>"
		}
		result += "</root>"
		return result
	}

	input := []byte(buildXML(100))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NewParser(input).Parse()
	}
}

func BenchmarkParserDeepNesting(b *testing.B) {
	var buildXML func(depth int) string
	buildXML = func(depth int) string {
		if depth == 0 {
			return "<leaf/>"
		}
		return "<a>" + buildXML(depth-1) + "</a>"
	}

	input := []byte(buildXML(50))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NewParser(input).Parse()
	}
}

func BenchmarkArenaAlloc(b *testing.B) {
	arena := NewArena()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = arena.Alloc(64)
	}
}

func BenchmarkArenaInternBytes(b *testing.B) {
	arena := NewArena()
	data := []byte("Hello World, this is a test string.")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = arena.InternBytes(data)
	}
}

func BenchmarkNodeString(b *testing.B) {
	input := []byte(`<root><a><b><c>text</c></b></a></root>`)
	doc, _ := NewParser(input).Parse()
	root := doc.FirstChild

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = root.String()
	}
}
