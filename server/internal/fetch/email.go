package fetch

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/http"
	"net/mail"
	"net/textproto"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html/charset"
)

// EmailMessage is a parsed RFC 822 message reduced to what the reading
// pipeline needs. Inline (cid:) images keep their raw bytes so they can be
// registered into the media cache before the HTML is sanitized.
type EmailMessage struct {
	MessageID   string
	FromName    string
	FromAddr    string
	Subject     string
	Date        time.Time
	HTML        string
	Text        string
	Inline      []EmailInlineImage
	BrowserLink string
}

type EmailInlineImage struct {
	CID         string
	ContentType string
	Data        []byte
}

// inlineImageLimit matches the media proxy cap (20 MB) per attachment.
const inlineImageLimit = 20 << 20

// ParseEmailMessage parses a raw RFC 822 message. It prefers the text/html
// part (multipart/alternative), collects multipart/related inline images and
// falls back to text/plain. A single malformed part never aborts parsing.
func ParseEmailMessage(raw []byte) (EmailMessage, error) {
	var result EmailMessage
	message, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return result, fmt.Errorf("邮件格式无效：%v", err)
	}
	header := message.Header
	result.MessageID = strings.Trim(strings.TrimSpace(header.Get("Message-Id")), "<>")
	if from, err := mail.ParseAddress(header.Get("From")); err == nil && from != nil {
		result.FromAddr = strings.ToLower(strings.TrimSpace(from.Address))
		result.FromName = strings.TrimSpace(from.Name)
	}
	if result.FromName == "" && result.FromAddr != "" {
		result.FromName = strings.SplitN(result.FromAddr, "@", 2)[0]
	}
	result.Subject = decodeEmailHeader(header.Get("Subject"))
	if date, err := mail.ParseDate(header.Get("Date")); err == nil {
		result.Date = date
	}
	if archived := strings.TrimSpace(header.Get("Archived-At")); archived != "" {
		result.BrowserLink = extractURLFromAngle(archived)
	}

	mediaType, params, err := mime.ParseMediaType(header.Get("Content-Type"))
	if err != nil {
		mediaType, params = "text/plain", map[string]string{}
	}
	var html, text string
	var inline []EmailInlineImage
	if strings.HasPrefix(mediaType, "multipart/") {
		html, text, inline = walkEmailMultipart(mediaType, params, message.Body)
	} else {
		data := decodeTransfer(header.Get("Content-Transfer-Encoding"), message.Body)
		text, html, inline = interpretEmailPart(mediaType, params["charset"], data, textproto.MIMEHeader(header))
	}
	result.HTML = html
	result.Text = text
	result.Inline = inline
	result.normalizeInline()
	if result.BrowserLink == "" {
		result.BrowserLink = findBrowserLinkInHTML(result.HTML)
	}
	return result, nil
}

// walkEmailMultipart recurses through multipart/alternative, multipart/related
// and friends. text/html wins over text/plain; other leaves are interpreted
// per content type (inline images are collected, everything else ignored).
func walkEmailMultipart(mediaType string, params map[string]string, body io.Reader) (html, text string, inline []EmailInlineImage) {
	if strings.TrimSpace(params["boundary"]) == "" {
		data := decodeTransfer("", body)
		text, _ = interpretEmailTextPart("text/plain", "utf-8", data)
		return "", text, nil
	}
	parts := multipart.NewReader(body, params["boundary"])
	for {
		part, err := parts.NextPart()
		if err != nil {
			return html, text, inline
		}
		partType, partParams, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
		if err != nil {
			partType, partParams = "application/octet-stream", map[string]string{}
		}
		if strings.HasPrefix(partType, "multipart/") {
			partHTML, partText, partInline := walkEmailMultipart(partType, partParams, part)
			html = firstNonEmpty(html, partHTML)
			if text == "" {
				text = partText
			}
			inline = append(inline, partInline...)
			continue
		}
		data := decodeTransfer(part.Header.Get("Content-Transfer-Encoding"), part)
		partText, partHTML, partInline := interpretEmailPart(partType, partParams["charset"], data, part.Header)
		if partHTML != "" {
			html = firstNonEmpty(html, partHTML)
		}
		if partText != "" && text == "" {
			text = partText
		}
		inline = append(inline, partInline...)
	}
}

// interpretEmailPart converts one MIME leaf into text, html or inline image.
func interpretEmailPart(mediaType, partCharset string, data []byte, header textproto.MIMEHeader) (text, html string, inline []EmailInlineImage) {
	if contentType, ok := inlineImageContentType(mediaType, header, data); ok {
		cid := strings.Trim(strings.TrimSpace(header.Get("Content-Id")), "<>")
		if cid == "" || len(data) == 0 || len(data) > inlineImageLimit {
			return "", "", nil
		}
		return "", "", []EmailInlineImage{{CID: cid, ContentType: contentType, Data: data}}
	}
	text, html = interpretEmailTextPart(mediaType, partCharset, data)
	return text, html, nil
}

// inlineImageContentType accepts image parts even when the sender mislabels
// them as application/octet-stream but still provides a Content-ID and valid
// PNG/JPEG/GIF bytes (seen in some bank newsletters).
func inlineImageContentType(mediaType string, header textproto.MIMEHeader, data []byte) (string, bool) {
	if strings.HasPrefix(mediaType, "image/") {
		return mediaType, true
	}
	if strings.TrimSpace(header.Get("Content-Id")) == "" || len(data) == 0 {
		return "", false
	}
	detected := http.DetectContentType(data)
	if strings.HasPrefix(detected, "image/") {
		return detected, true
	}
	return "", false
}

func interpretEmailTextPart(mediaType, partCharset string, data []byte) (text, html string) {
	switch mediaType {
	case "text/html":
		return "", decodePartCharset(partCharset, data)
	case "text/plain":
		return decodePartCharset(partCharset, data), ""
	}
	return "", ""
}

func decodeTransfer(encoding string, body io.Reader) []byte {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "base64":
		data, err := io.ReadAll(io.LimitReader(body, inlineImageLimit+1))
		if err != nil {
			return nil
		}
		cleaned := emailBase64Whitespace.ReplaceAllLiteralString(string(data), "")
		decoded, err := base64.StdEncoding.DecodeString(cleaned)
		if err != nil {
			// Some mailers emit slightly broken base64; keep whatever decoded.
			decoded, _ = base64.StdEncoding.DecodeString(strings.TrimSpace(cleaned))
		}
		return decoded
	case "quoted-printable":
		decoded, err := io.ReadAll(quotedprintable.NewReader(body))
		if err != nil {
			return decoded
		}
		return decoded
	default:
		data, _ := io.ReadAll(io.LimitReader(body, inlineImageLimit+1))
		return data
	}
}

var emailBase64Whitespace = regexp.MustCompile(`[\s]+`)

func decodePartCharset(label string, data []byte) string {
	if utf8.Valid(data) {
		return string(data)
	}
	if strings.TrimSpace(label) == "" {
		label = "utf-8"
	}
	reader, err := charset.NewReaderLabel(label, bytes.NewReader(data))
	if err != nil {
		return string(data)
	}
	decoded, err := io.ReadAll(io.LimitReader(reader, inlineImageLimit+1))
	if err != nil {
		return string(data)
	}
	return string(decoded)
}

var emailWordDecoder = &mime.WordDecoder{
	CharsetReader: func(charsetLabel string, input io.Reader) (io.Reader, error) {
		return charset.NewReaderLabel(charsetLabel, input)
	},
}

func decodeEmailHeader(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if decoded, err := emailWordDecoder.DecodeHeader(value); err == nil {
		return strings.TrimSpace(decoded)
	}
	return value
}

func extractURLFromAngle(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "<") && strings.HasSuffix(value, ">") {
		value = value[1 : len(value)-1]
	}
	return strings.TrimSpace(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// browserLinkPhrases covers the common "open in browser" patterns used by
// Substack, Mailchimp and 163 newsletter mail.
var browserLinkPhrases = []string{
	"在浏览器中阅读", "在浏览器中查看", "在浏览器打开", "浏览器中查看",
	"view in browser", "view this email", "open in browser", "read online", "read in browser",
}

// findBrowserLinkInHTML returns the first http(s) link whose anchor text looks
// like a "view in browser" affordance.
func findBrowserLinkInHTML(html string) string {
	if strings.TrimSpace(html) == "" {
		return ""
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return ""
	}
	var link string
	doc.Find("a[href]").Each(func(_ int, selection *goquery.Selection) {
		if link != "" {
			return
		}
		href := strings.TrimSpace(selection.AttrOr("href", ""))
		if !isHTTPURL(href) {
			return
		}
		text := strings.ToLower(strings.TrimSpace(selection.Text()))
		for _, phrase := range browserLinkPhrases {
			if strings.Contains(text, phrase) || strings.Contains(strings.ToLower(href), phrase) {
				link = href
				return
			}
		}
	})
	return link
}

func (m *EmailMessage) normalizeInline() {
	seen := make(map[string]struct{}, len(m.Inline))
	clean := make([]EmailInlineImage, 0, len(m.Inline))
	for _, image := range m.Inline {
		image.CID = strings.Trim(strings.TrimSpace(image.CID), "<>")
		if image.CID == "" || len(image.Data) == 0 {
			continue
		}
		if _, exists := seen[image.CID]; exists {
			continue
		}
		seen[image.CID] = struct{}{}
		clean = append(clean, image)
	}
	m.Inline = clean
}

// EmailInlinePseudoURL builds the stable synthetic URL an inline image is
// referenced by before sanitization. SanitizeContent rewrites it into
// /media/{token}/{hash}; the media cache row for that hash is pre-written so
// the bytes are served locally and never fetched.
func EmailInlinePseudoURL(messageID, cid string) string {
	return fmt.Sprintf("https://rill-email-inline.invalid/%s/%s", URLHash(messageID), URLHash(cid))
}

// BuildEmailFeedItem converts a parsed email into the storage-ready item shape.
func BuildEmailFeedItem(message EmailMessage, sourceToken string) FeedItem {
	now := time.Now().UTC()
	published := message.Date
	if published.IsZero() {
		published = now
	}
	published = published.UTC()
	rawHTML := message.HTML
	if strings.TrimSpace(rawHTML) == "" && strings.TrimSpace(message.Text) != "" {
		rawHTML = textToParagraphs(message.Text)
	}
	// Rewrite cid: references to synthetic absolute URLs first: SanitizeContent
	// strips non-http image sources, so the substitution must happen before it.
	for _, image := range message.Inline {
		pseudo := EmailInlinePseudoURL(message.MessageID, image.CID)
		rawHTML = replaceCIDReference(rawHTML, image.CID, pseudo)
	}
	var emptyURL url.URL
	link := message.BrowserLink
	contentHTML, contentText, mediaURLs := SanitizeEmailContent(rawHTML, link, &emptyURL, sourceToken)
	if contentText == "" && strings.TrimSpace(message.Text) != "" {
		contentText = collapseWhitespace(message.Text)
	}
	title := strings.TrimSpace(message.Subject)
	if title == "" {
		title = firstLine(contentText)
	}
	if title == "" {
		title = "(无标题邮件)"
	}
	if len(title) > 500 {
		title = truncateRunes(title, 500)
	}
	guid := message.MessageID
	if guid == "" {
		guid = hashString(message.FromAddr + "\n" + title + "\n" + published.Format(time.RFC3339Nano))
	}
	hash := hashString(guid + "\n" + title + "\n" + contentText + "\n" + strings.Join(mediaURLs, "\n"))
	return FeedItem{
		GUID:               guid,
		URL:                link,
		Title:              title,
		Author:             message.FromName,
		PublishedAt:        published,
		PublishedEstimated: message.Date.IsZero(),
		ContentHTML:        contentHTML,
		ContentText:        contentText,
		ContentHash:        hash,
		MediaURLs:          mediaURLs,
		EmailSender:        message.FromAddr,
	}
}

func textToParagraphs(text string) string {
	paragraphs := strings.Split(strings.TrimSpace(text), "\n")
	for i, paragraph := range paragraphs {
		trimmed := strings.TrimSpace(paragraph)
		if trimmed == "" {
			continue
		}
		paragraphs[i] = "<p>" + htmlEscape(trimmed) + "</p>"
	}
	return strings.Join(paragraphs, "\n")
}

func htmlEscape(value string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&#34;", "'", "&#39;").Replace(value)
}

// replaceCIDReference swaps one cid: reference for the synthetic absolute URL.
// Case-insensitive on the scheme; references appear like `cid:image001.png@01D`.
func replaceCIDReference(html, cid, pseudoURL string) string {
	lower := strings.ToLower(html)
	lowerCID := strings.ToLower(strings.TrimSpace(cid))
	needles := []string{
		"cid:" + lowerCID,
		"cid:" + strings.ReplaceAll(lowerCID, "@", "%40"),
	}
	earliest := -1
	match := ""
	for _, needle := range needles {
		if idx := strings.Index(lower, needle); idx >= 0 && (earliest < 0 || idx < earliest) {
			earliest = idx
			match = needle
		}
	}
	if earliest < 0 {
		return html
	}
	return html[:earliest] + pseudoURL + replaceCIDReference(html[earliest+len(match):], cid, pseudoURL)
}
