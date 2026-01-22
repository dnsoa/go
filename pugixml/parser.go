package pugixml

import (
	"bytes"
	"fmt"
	"unicode/utf8"
)

// Parser 解析器结构
type Parser struct {
	arena *ByteArena
	buf   []byte
	pos   int
	line  int // 用于错误报告
	col   int
}

// NewParser 创建新的解析器
func NewParser(input []byte) *Parser {
	return &Parser{
		arena: NewArena(),
		buf:   input,
		line:  1,
		col:   1,
	}
}

// Parse 解析 XML 并返回文档根节点
func (p *Parser) Parse() (*Node, error) {
	doc := AllocNode(p.arena)
	doc.Type = NodeDocument

	for p.pos < len(p.buf) {
		p.skipWS()
		if p.pos >= len(p.buf) {
			break
		}

		if p.buf[p.pos] == '<' {
			p.pos++
			if err := p.parseMarkup(doc); err != nil {
				return nil, err
			}
		} else {
			// 顶层文本内容
			textStart := p.pos
			for p.pos < len(p.buf) && p.buf[p.pos] != '<' {
				p.advance()
			}
			if textStart < p.pos {
				text := p.strconvInSitu(p.buf[textStart:p.pos])
				if len(text) > 0 {
					textNode := AllocNode(p.arena)
					textNode.Type = NodePCDATA
					textNode.Value = p.arena.InternBytes(text)
					doc.AppendChild(p.arena, textNode)
				}
			}
		}
	}
	return doc, nil
}

// parseMarkup 解析标记（标签、注释、CDATA等）
func (p *Parser) parseMarkup(parent *Node) error {
	if p.pos >= len(p.buf) {
		return p.error("unexpected EOF")
	}

	switch p.buf[p.pos] {
	case '!':
		return p.parseSpecial(parent)
	case '?':
		return p.parsePI(parent)
	case '/':
		return p.parseClosingTag(parent)
	default:
		return p.parseElement(parent)
	}
}

// parseSpecial 解析特殊标记（注释、CDATA）
func (p *Parser) parseSpecial(parent *Node) error {
	p.pos++
	// 注释以 "--" 开头
	if p.pos+1 < len(p.buf) && bytes.HasPrefix(p.buf[p.pos:], []byte("--")) {
		return p.parseComment(parent)
	}
	// CDATA 以 "[CDATA[" 开头
	if bytes.HasPrefix(p.buf[p.pos:], []byte("[CDATA[")) {
		return p.parseCDATA(parent)
	}
	return p.skipUntil('>')
}

// parseComment 解析注释
func (p *Parser) parseComment(parent *Node) error {
	p.pos += 2 // 跳过 '--'
	start := p.pos

	for p.pos < len(p.buf) {
		if bytes.HasPrefix(p.buf[p.pos:], []byte("-->")) {
			content := p.buf[start:p.pos]
			node := AllocNode(p.arena)
			node.Type = NodeComment
			node.Value = p.arena.InternBytes(content)
			parent.AppendChild(p.arena, node)
			p.pos += 3
			return nil
		}
		p.advance()
	}
	return p.error("unterminated comment")
}

// parseCDATA 解析 CDATA 段
func (p *Parser) parseCDATA(parent *Node) error {
	p.pos += 7 // 跳过 '[CDATA['
	start := p.pos

	for p.pos < len(p.buf) {
		if bytes.HasPrefix(p.buf[p.pos:], []byte("]]>")) {
			content := p.buf[start:p.pos]
			node := AllocNode(p.arena)
			node.Type = NodeCDATA
			node.Value = p.arena.InternBytes(content)
			parent.AppendChild(p.arena, node)
			p.pos += 3
			return nil
		}
		p.advance()
	}
	return p.error("unterminated CDATA section")
}

// parsePI 解析处理指令
func (p *Parser) parsePI(parent *Node) error {
	p.pos++ // 跳过 '?'
	start := p.pos

	for p.pos < len(p.buf) {
		if bytes.HasPrefix(p.buf[p.pos:], []byte("?>")) {
			content := p.buf[start:p.pos]
			node := AllocNode(p.arena)
			node.Type = NodePI
			node.Value = p.arena.InternBytes(content)
			parent.AppendChild(p.arena, node)
			p.pos += 2
			return nil
		}
		p.advance()
	}
	return p.error("unterminated processing instruction")
}

// parseClosingTag 解析结束标签
func (p *Parser) parseClosingTag(parent *Node) error {
	p.pos++ // 跳过 '/'
	p.skipWS()

	nameStart := p.pos
	for p.pos < len(p.buf) && !isSpace(p.buf[p.pos]) && p.buf[p.pos] != '>' {
		p.pos++
	}
	name := p.buf[nameStart:p.pos]

	p.skipWS()
	if p.pos >= len(p.buf) || p.buf[p.pos] != '>' {
		return p.error("expected '>' in closing tag")
	}
	p.pos++

	// 验证结束标签匹配
	if parent.Type == NodeElement && !bytes.Equal(parent.Name, name) {
		return p.error(fmt.Sprintf("mismatched closing tag: expected </%s>, got </%s>", parent.Name, name))
	}

	return nil
}

// parseElement 解析元素
func (p *Parser) parseElement(parent *Node) error {
	node := AllocNode(p.arena)
	node.Type = NodeElement

	// 解析元素名
	nameStart := p.pos
	for p.pos < len(p.buf) && !isSpace(p.buf[p.pos]) && p.buf[p.pos] != '>' && p.buf[p.pos] != '/' {
		p.pos++
	}
	if nameStart == p.pos {
		return p.error("empty element name")
	}
	node.Name = p.arena.InternBytes(p.buf[nameStart:p.pos])

	// 解析属性
	for {
		p.skipWS()
		if p.pos >= len(p.buf) || p.buf[p.pos] == '>' || p.buf[p.pos] == '/' {
			break
		}
		if err := p.parseAttribute(node); err != nil {
			return err
		}
	}

	// 检查自闭合标签
	if p.pos+1 < len(p.buf) && p.buf[p.pos] == '/' && p.buf[p.pos+1] == '>' {
		p.pos += 2 // 跳过 '/>'
		parent.AppendChild(p.arena, node)
		return nil
	}

	if p.pos >= len(p.buf) || p.buf[p.pos] != '>' {
		return p.error("expected '>' or '/>'")
	}
	p.pos++ // 跳过 '>'

	// 解析子节点
	for p.pos < len(p.buf) {
		p.skipWS()
		if p.pos >= len(p.buf) {
			return p.error("unexpected EOF in element")
		}

		if p.buf[p.pos] == '<' {
			p.pos++
			if p.pos < len(p.buf) && p.buf[p.pos] == '/' {
				// 结束标签
				p.pos++
				p.skipWS()
				nameStart := p.pos
				for p.pos < len(p.buf) && !isSpace(p.buf[p.pos]) && p.buf[p.pos] != '>' {
					p.pos++
				}
				closingName := p.buf[nameStart:p.pos]
				p.skipWS()
				if p.pos >= len(p.buf) || p.buf[p.pos] != '>' {
					return p.error("expected '>' in closing tag")
				}
				p.pos++

				if !bytes.Equal(node.Name, closingName) {
					return p.error(fmt.Sprintf("mismatched closing tag: expected </%s>, got </%s>", node.Name, closingName))
				}
				break
			}
			if err := p.parseMarkup(node); err != nil {
				return err
			}
		} else {
			// 文本内容
			textStart := p.pos
			for p.pos < len(p.buf) && p.buf[p.pos] != '<' {
				p.advance()
			}
			if textStart < p.pos {
				text := p.strconvInSitu(p.buf[textStart:p.pos])
				if len(text) > 0 {
					textNode := AllocNode(p.arena)
					textNode.Type = NodePCDATA
					textNode.Value = p.arena.InternBytes(text)
					node.AppendChild(p.arena, textNode)
				}
			}
		}
	}

	parent.AppendChild(p.arena, node)
	return nil
}

// parseAttribute 解析属性
func (p *Parser) parseAttribute(node *Node) error {
	// 解析属性名
	nameStart := p.pos
	for p.pos < len(p.buf) && !isSpace(p.buf[p.pos]) && p.buf[p.pos] != '=' && p.buf[p.pos] != '>' {
		p.pos++
	}
	if nameStart == p.pos {
		return p.error("empty attribute name")
	}
	attrName := p.buf[nameStart:p.pos]

	p.skipWS()

	// 检查是否有属性值
	var attrValue []byte
	if p.pos < len(p.buf) && p.buf[p.pos] == '=' {
		p.pos++
		p.skipWS()

		if p.pos >= len(p.buf) {
			return p.error("unexpected EOF after '='")
		}

		quote := p.buf[p.pos]
		if quote != '"' && quote != '\'' {
			return p.error("attribute value must be quoted")
		}
		p.pos++

		valueStart := p.pos
		for p.pos < len(p.buf) && p.buf[p.pos] != quote {
			if p.buf[p.pos] == '<' {
				return p.error("'<' not allowed in attribute value")
			}
			p.advance()
		}
		if p.pos >= len(p.buf) {
			return p.error("unterminated attribute value")
		}
		attrValue = p.buf[valueStart:p.pos]
		p.pos++ // 跳过结束引号
	}

	attr := AllocAttr(p.arena)
	attr.Name = p.arena.InternBytes(attrName)
	attr.Value = p.arena.InternBytes(p.strconvInSitu(attrValue))
	node.AppendAttr(p.arena, attr)

	return nil
}

// strconvInSitu 原地处理字符和实体引用转义
// 如果不需要转义，直接返回原切片（零分配）
func (p *Parser) strconvInSitu(s []byte) []byte {
	// 查找是否需要处理
	needProcess := false
	for _, b := range s {
		if b == '&' || b == '\r' {
			needProcess = true
			break
		}
	}
	if !needProcess {
		return s
	}

	// 创建处理后的切片
	result := make([]byte, 0, len(s))
	i := 0
	for i < len(s) {
		if s[i] == '\r' {
			result = append(result, '\n')
			i++
			if i < len(s) && s[i] == '\n' {
				i++
			}
			continue
		}
		if s[i] == '&' {
			semi := bytes.IndexByte(s[i:], ';')
			if semi > 0 {
				ent := s[i+1 : i+semi]
				if val, ok := parseEntity(ent); ok {
					result = append(result, val...)
					i += semi + 1
					continue
				}
			}
		}
		result = append(result, s[i])
		i++
	}
	return result
}

// parseEntity 解析 XML 实体引用
func parseEntity(ent []byte) ([]byte, bool) {
	if len(ent) == 0 {
		return nil, false
	}

	switch {
	case len(ent) == 2 && ent[0] == 'l' && ent[1] == 't':
		return []byte{'<'}, true
	case len(ent) == 2 && ent[0] == 'g' && ent[1] == 't':
		return []byte{'>'}, true
	case len(ent) == 3 && ent[0] == 'a' && ent[1] == 'm' && ent[2] == 'p':
		return []byte{'&'}, true
	case len(ent) == 4 && ent[0] == 'a' && ent[1] == 'p' && ent[2] == 'o' && ent[3] == 's':
		return []byte{'\''}, true
	case len(ent) == 4 && ent[0] == 'q' && ent[1] == 'u' && ent[2] == 'o' && ent[3] == 't':
		return []byte{'"'}, true
	default:
		// 数字实体引用 &#123; 或 &#x1F600;
		if len(ent) > 0 && ent[0] == '#' {
			var num int64
			if len(ent) > 1 && (ent[1] == 'x' || ent[1] == 'X') {
				// 十六进制
				for _, c := range ent[2:] {
					num <<= 4
					switch {
					case c >= '0' && c <= '9':
						num += int64(c - '0')
					case c >= 'a' && c <= 'f':
						num += int64(c - 'a' + 10)
					case c >= 'A' && c <= 'F':
						num += int64(c - 'A' + 10)
					default:
						return nil, false
					}
				}
			} else {
				// 十进制
				for _, c := range ent[1:] {
					if c < '0' || c > '9' {
						return nil, false
					}
					num = num*10 + int64(c-'0')
				}
			}
			// 将数值转换为 UTF-8 编码的字节序列
			if num >= 0 {
				r := rune(num)
				buf := make([]byte, 4)
				n := utf8.EncodeRune(buf, r)
				return buf[:n], true
			}
			return nil, false
		}
	}
	return nil, false
}

func (p *Parser) skipWS() {
	for p.pos < len(p.buf) && isSpace(p.buf[p.pos]) {
		p.advance()
	}
}

func (p *Parser) skipUntil(b byte) error {
	for p.pos < len(p.buf) && p.buf[p.pos] != b {
		p.advance()
	}
	if p.pos < len(p.buf) {
		p.pos++
		return nil
	}
	return p.error(fmt.Sprintf("unexpected EOF, expected '%c'", b))
}

func (p *Parser) advance() {
	if p.pos < len(p.buf) {
		if p.buf[p.pos] == '\n' {
			p.line++
			p.col = 1
		} else {
			p.col++
		}
		p.pos++
	}
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

func (p *Parser) error(msg string) error {
	return fmt.Errorf("parse error at line %d, col %d: %s", p.line, p.col, msg)
}
