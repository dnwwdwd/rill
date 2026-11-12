// Package markdown converts stored article HTML into readable Markdown for the
// copy/download endpoint and the MCP get_item tool. It intentionally supports a
// small, predictable subset: headings, paragraphs, lists, quotes, code, links,
// images and basic inline emphasis. Unknown elements fall back to their text.
package markdown

import (
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

const maxBlankLines = 2

// FromHTML converts article HTML into Markdown. baseURL resolves relative links
// and image sources; empty baseURL leaves them unchanged.
func FromHTML(raw, baseURL string) string {
	return FromHTMLWithRoot(raw, baseURL, "")
}

// FromHTMLWithRoot is FromHTML with a site-root base. Root-relative URLs such as
// Rill's /media/{token}/{hash} images are resolved against rootURL instead of
// the article URL, so exported Markdown keeps working media links.
func FromHTMLWithRoot(raw, baseURL, rootURL string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(raw))
	if err != nil {
		return strings.TrimSpace(raw)
	}
	rootURL = strings.TrimRight(strings.TrimSpace(rootURL), "/")
	if rootURL != "" {
		doc.Find("[src]").Each(func(_ int, selection *goquery.Selection) {
			if value, ok := selection.Attr("src"); ok && strings.HasPrefix(value, "/media/") {
				selection.SetAttr("src", rootURL+value)
			}
		})
		doc.Find("[href]").Each(func(_ int, selection *goquery.Selection) {
			if value, ok := selection.Attr("href"); ok && strings.HasPrefix(value, "/media/") {
				selection.SetAttr("href", rootURL+value)
			}
		})
	}
	var out strings.Builder
	renderChildren(&out, doc.Find("body").Get(0), baseURL)
	return normalize(out.String())
}

func renderChildren(out *strings.Builder, node *html.Node, baseURL string) {
	if node == nil {
		return
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		renderNode(out, child, baseURL)
	}
}

func renderNode(out *strings.Builder, node *html.Node, baseURL string) {
	switch node.Type {
	case html.TextNode:
		out.WriteString(escapeText(collapseSpaces(node.Data)))
		return
	case html.ElementNode:
		// continue below
	default:
		return
	}

	tag := strings.ToLower(node.Data)
	switch tag {
	case "script", "style", "noscript", "svg", "head", "template":
		return
	case "h1", "h2", "h3", "h4", "h5", "h6":
		level := int(tag[1] - '0')
		text := strings.TrimSpace(renderInline(node, baseURL))
		if text != "" {
			block(out, strings.Repeat("#", level)+" "+text)
		}
	case "p":
		text := strings.TrimSpace(renderInline(node, baseURL))
		if text != "" {
			block(out, text)
		}
	case "br":
		out.WriteString("\n")
	case "hr":
		block(out, "---")
	case "blockquote":
		text := strings.TrimSpace(renderBlockChildren(node, baseURL))
		if text != "" {
			lines := strings.Split(text, "\n")
			for i, line := range lines {
				lines[i] = "> " + line
			}
			block(out, strings.Join(lines, "\n"))
		}
	case "ul", "ol":
		text := renderList(node, baseURL)
		if text != "" {
			block(out, text)
		}
	case "pre":
		text := strings.TrimRight(nodeText(node), "\n")
		if text != "" {
			fence := "```"
			if strings.Contains(text, "```") {
				fence = "````"
			}
			block(out, fence+"\n"+text+"\n"+fence)
		}
	case "table":
		text := renderTable(node, baseURL)
		if text != "" {
			block(out, text)
		}
	case "div", "section", "article", "main", "header", "footer", "figure", "figcaption", "details", "summary":
		text := strings.TrimSpace(renderBlockChildren(node, baseURL))
		if text != "" {
			block(out, text)
		}
	default:
		out.WriteString(renderInline(node, baseURL))
	}
}

func renderInline(node *html.Node, baseURL string) string {
	var out strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		switch child.Type {
		case html.TextNode:
			out.WriteString(escapeText(collapseSpaces(child.Data)))
		case html.ElementNode:
			renderInlineElement(&out, child, baseURL)
		}
	}
	return out.String()
}

func renderInlineElement(out *strings.Builder, node *html.Node, baseURL string) {
	tag := strings.ToLower(node.Data)
	switch tag {
	case "script", "style", "noscript", "svg", "template":
		return
	case "br":
		out.WriteString("\n")
	case "strong", "b":
		inner := strings.TrimSpace(renderInline(node, baseURL))
		if inner != "" {
			out.WriteString("**" + inner + "**")
		}
	case "em", "i":
		inner := strings.TrimSpace(renderInline(node, baseURL))
		if inner != "" {
			out.WriteString("*" + inner + "*")
		}
	case "del", "s", "strike":
		inner := strings.TrimSpace(renderInline(node, baseURL))
		if inner != "" {
			out.WriteString("~~" + inner + "~~")
		}
	case "code":
		inner := strings.ReplaceAll(nodeText(node), "`", "\\`")
		if inner != "" {
			out.WriteString("`" + inner + "`")
		}
	case "a":
		text := strings.TrimSpace(renderInline(node, baseURL))
		href := resolveURL(attr(node, "href"), baseURL)
		if text == "" {
			return
		}
		if href == "" {
			out.WriteString(text)
			return
		}
		out.WriteString("[" + text + "](" + href + ")")
	case "img":
		src := resolveURL(attr(node, "src"), baseURL)
		if src == "" {
			return
		}
		alt := escapeText(strings.TrimSpace(attr(node, "alt")))
		out.WriteString("![" + alt + "](" + src + ")")
	default:
		out.WriteString(renderInline(node, baseURL))
	}
}

func renderBlockChildren(node *html.Node, baseURL string) string {
	var out strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		renderNode(&out, child, baseURL)
	}
	return out.String()
}

func renderList(node *html.Node, baseURL string) string {
	ordered := strings.EqualFold(node.Data, "ol")
	index := 0
	var lines []string
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode || !strings.EqualFold(child.Data, "li") {
			continue
		}
		index++
		text := strings.TrimSpace(renderBlockChildren(child, baseURL))
		if text == "" {
			continue
		}
		lines = append(lines, listText(text, ordered, index))
	}
	return strings.Join(lines, "\n")
}

func listText(text string, ordered bool, index int) string {
	lines := strings.Split(text, "\n")
	prefix := "- "
	if ordered {
		prefix = itoa(index) + ". "
	}
	for i := range lines {
		if i == 0 {
			lines[i] = prefix + lines[i]
			continue
		}
		lines[i] = strings.Repeat(" ", len(prefix)) + lines[i]
	}
	return strings.Join(lines, "\n")
}

func renderTable(node *html.Node, baseURL string) string {
	var lines []string
	for row := node.FirstChild; row != nil; row = row.NextSibling {
		if row.Type != html.ElementNode || !strings.EqualFold(row.Data, "tr") {
			continue
		}
		var cells []string
		for cell := row.FirstChild; cell != nil; cell = cell.NextSibling {
			if cell.Type != html.ElementNode || (!strings.EqualFold(cell.Data, "td") && !strings.EqualFold(cell.Data, "th")) {
				continue
			}
			cells = append(cells, strings.TrimSpace(renderInline(cell, baseURL)))
		}
		if len(cells) > 0 {
			lines = append(lines, "| "+strings.Join(cells, " | ")+" |")
		}
	}
	if len(lines) == 0 {
		return ""
	}
	// Insert a Markdown separator after the first row.
	cols := strings.Count(lines[0], "|") - 1
	if cols > 0 {
		separator := make([]string, cols)
		for i := range separator {
			separator[i] = "---"
		}
		lines = append(lines[:1], append([]string{"| " + strings.Join(separator, " | ") + " |"}, lines[1:]...)...)
	}
	return strings.Join(lines, "\n")
}

func block(out *strings.Builder, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if out.Len() > 0 && !strings.HasSuffix(out.String(), "\n\n") {
		out.WriteString("\n\n")
	}
	out.WriteString(text)
	out.WriteString("\n\n")
}

func normalize(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	lines := strings.Split(value, "\n")
	result := make([]string, 0, len(lines))
	blank := 0
	for _, line := range lines {
		line = strings.TrimRight(line, " \t")
		if strings.TrimSpace(line) == "" {
			blank++
			if blank > maxBlankLines {
				continue
			}
		} else {
			blank = 0
		}
		result = append(result, line)
	}
	return strings.TrimSpace(strings.Join(result, "\n"))
}

func nodeText(node *html.Node) string {
	if node == nil {
		return ""
	}
	var out strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			out.WriteString(n.Data)
			return
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return out.String()
}

func attr(node *html.Node, name string) string {
	for _, item := range node.Attr {
		if strings.EqualFold(item.Key, name) {
			return item.Val
		}
	}
	return ""
}

func resolveURL(value, baseURL string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if baseURL == "" {
		return value
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return value
	}
	target, err := url.Parse(value)
	if err != nil {
		return value
	}
	return base.ResolveReference(target).String()
}

func collapseSpaces(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func escapeText(value string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		"`", "\\`",
		"*", `\*`,
		"_", `\_`,
		"[", `\[`,
		"]", `\]`,
	)
	// Preserve intentional line breaks from <br>.
	if strings.Contains(value, "\n") {
		parts := strings.Split(value, "\n")
		for i := range parts {
			parts[i] = replacer.Replace(parts[i])
		}
		return strings.Join(parts, "\n")
	}
	return replacer.Replace(value)
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [20]byte
	pos := len(digits)
	for value > 0 {
		pos--
		digits[pos] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[pos:])
}
