package fetch

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"golang.org/x/net/html/charset"
)

// IMAPConfig is one mailbox connection. Password is the account's client
// authorization code (授权码), never the web login password.
type IMAPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
}

const (
	// FetchEmailBatchLimit 是配置允许的最大单轮同步封数；每个账号可在
	// 10–500 之间自行设置，默认 60。
	FetchEmailBatchLimit = 500
	// EmailPerFolderLimit keeps one busy folder (usually INBOX) from starving
	// the other folders in the same poll.
	EmailPerFolderLimit = 100
	imapDialTimeout     = 15 * time.Second
	// 多文件夹逐轮拉取的整体上限：服务器挂起时看门狗会关连接，调度不被拖死。
	imapFetchTimeout = 120 * time.Second
	// 测试连接上限：设置页按钮要在可接受时间内给出结论。
	imapListTimeout = 12 * time.Second
)

// FetchedEmail is one raw message together with the folder it came from.
type FetchedEmail struct {
	UID     uint32
	Mailbox string
	Raw     []byte
}

// EmailFolderState tracks IMAP UIDVALIDITY/UID per folder so polling can be
// incremental and does not depend on the \Seen flag (mailbox auto-sync must not
// mutate the user's read state).
type EmailFolderState struct {
	UIDValidity uint32 `json:"uid_validity"`
	LastUID     uint32 `json:"last_uid"`
}

type EmailSyncState struct {
	Username string                      `json:"username"`
	Folders  map[string]EmailFolderState `json:"folders"`
}

type EmailSyncResult struct {
	Emails   []FetchedEmail
	State    EmailSyncState
	Warnings []error
	// Folders is the number of folders that were read successfully.
	Folders int
}

// dialIMAPAddr mirrors fetch.Client's IPv4-first dialing: some microserver/LAN
// DNS setups return a non-routable IPv6 address that accepts TCP then drops or
// blackholes, consuming the whole dial timeout before IPv4 is even tried.
// Try A records first, then AAAA fallbacks.
func dialIMAPAddr(ctx context.Context, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil || net.ParseIP(host) != nil {
		dialer := &net.Dialer{Timeout: imapDialTimeout}
		return dialer.DialContext(ctx, "tcp", address)
	}
	resolved, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	ordered := make([]net.IPAddr, 0, len(resolved))
	for _, addr := range resolved {
		if addr.IP.To4() != nil {
			ordered = append(ordered, addr)
		}
	}
	for _, addr := range resolved {
		if addr.IP.To4() == nil {
			ordered = append(ordered, addr)
		}
	}
	if len(ordered) == 0 {
		return nil, fmt.Errorf("no addresses for %s", host)
	}
	var lastErr error
	for _, addr := range ordered {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		dialer := &net.Dialer{Timeout: imapDialTimeout}
		conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(addr.IP.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func imapAddress(cfg IMAPConfig) string {
	port := cfg.Port
	if port <= 0 {
		port = 993
	}
	return net.JoinHostPort(strings.TrimSpace(cfg.Host), fmt.Sprint(port))
}

// selectMailbox opens the requested mailbox. Mail clients often show a folder
// by its leaf name (e.g. "Email") while the IMAP server exposes the full path
// (e.g. "其他文件夹/Email"). A plain SELECT of the typed name then returns
// "NO [NONEXISTENT] Folder not exist".
//
// On NONEXISTENT we LIST the account once and retry with:
//  1. a case-insensitive exact match, then
//  2. a unique leaf-name match (split by the server's hierarchy delimiter).
//
// Ambiguous leaf names are rejected instead of silently choosing one.
func selectMailbox(client *imapclient.Client, requested string) (string, error) {
	name, _, err := selectMailboxData(client, requested)
	return name, err
}

func selectMailboxData(client *imapclient.Client, requested string) (string, *imap.SelectData, error) {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		requested = "INBOX"
	}
	if data, err := client.Select(requested, nil).Wait(); err == nil {
		return requested, data, nil
	} else if !isNonExistentMailbox(err) {
		return "", nil, err
	}
	items, listErr := client.List("", "*", nil).Collect()
	if listErr != nil {
		// Keep the original SELECT error; it is the actionable one.
		return "", nil, fmt.Errorf("文件夹 %q 不存在，且获取文件夹列表失败：%v", requested, listErr)
	}
	resolved, candidates := resolveMailboxName(requested, items)
	switch {
	case resolved != "":
		data, err := client.Select(resolved, nil).Wait()
		if err != nil {
			return "", nil, err
		}
		return resolved, data, nil
	case len(candidates) > 1:
		return "", nil, fmt.Errorf("文件夹 %q 匹配到多个路径（%s），请在设置中选择完整路径", requested, strings.Join(candidates, "、"))
	default:
		return "", nil, fmt.Errorf("文件夹 %q 不存在，可用文件夹：%s", requested, strings.Join(mailboxNames(items), "、"))
	}
}

func isNonExistentMailbox(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToUpper(err.Error())
	return strings.Contains(message, "NONEXISTENT") || strings.Contains(message, "NOT EXIST")
}

func resolveMailboxName(requested string, items []*imap.ListData) (string, []string) {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		return "", nil
	}
	var exact string
	leaves := make([]string, 0)
	for _, item := range items {
		if item == nil || item.Mailbox == "" || hasMailboxAttr(item, imap.MailboxAttrNoSelect) {
			continue
		}
		if strings.EqualFold(item.Mailbox, requested) {
			exact = item.Mailbox
			continue
		}
		if item.Delim == 0 {
			continue
		}
		parts := strings.Split(item.Mailbox, string(item.Delim))
		if len(parts) > 1 && strings.EqualFold(parts[len(parts)-1], requested) {
			leaves = append(leaves, item.Mailbox)
		}
	}
	if exact != "" {
		return exact, nil
	}
	if len(leaves) == 1 {
		return leaves[0], nil
	}
	return "", leaves
}

func hasMailboxAttr(item *imap.ListData, target imap.MailboxAttr) bool {
	for _, attr := range item.Attrs {
		if strings.EqualFold(string(attr), string(target)) {
			return true
		}
	}
	return false
}

func mailboxNames(items []*imap.ListData) []string {
	names := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item == nil || item.Mailbox == "" || hasMailboxAttr(item, imap.MailboxAttrNoSelect) {
			continue
		}
		if _, ok := seen[item.Mailbox]; ok {
			continue
		}
		seen[item.Mailbox] = struct{}{}
		names = append(names, item.Mailbox)
	}
	return names
}

// syncableMailboxes lists folders that should be mirrored into Rill. System
// mailboxes (sent/drafts/trash/junk and virtual "all mail"/flagged views) are
// skipped so the reader does not import drafts or duplicate archived mail.
func syncableMailboxes(items []*imap.ListData) []*imap.ListData {
	result := make([]*imap.ListData, 0, len(items))
	for _, item := range items {
		if isSystemMailbox(item) {
			continue
		}
		result = append(result, item)
	}
	sort.SliceStable(result, func(i, j int) bool {
		iInbox := strings.EqualFold(result[i].Mailbox, "INBOX")
		jInbox := strings.EqualFold(result[j].Mailbox, "INBOX")
		if iInbox != jInbox {
			return iInbox
		}
		return strings.ToLower(result[i].Mailbox) < strings.ToLower(result[j].Mailbox)
	})
	return result
}

func isSystemMailbox(item *imap.ListData) bool {
	if item == nil || item.Mailbox == "" {
		return true
	}
	if hasMailboxAttr(item, imap.MailboxAttrNoSelect) {
		return true
	}
	for _, attr := range item.Attrs {
		switch strings.ToLower(string(attr)) {
		case "\\sent", "\\drafts", "\\trash", "\\junk", "\\all", "\\flagged", "\\important":
			return true
		}
	}
	name := item.Mailbox
	if item.Delim != 0 {
		parts := strings.Split(name, string(item.Delim))
		name = parts[len(parts)-1]
	}
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "sent", "sent items", "sent messages", "sent mail",
		"drafts", "draft",
		"trash", "deleted items", "deleted messages", "trash bin", "bin",
		"junk", "spam", "junk e-mail", "junk mail",
		"all mail", "all",
		"已发送", "已发送邮件", "发件箱", "草稿", "草稿箱",
		"已删除", "已删除邮件", "废纸篓", "垃圾桶",
		"垃圾邮件", "广告邮件", "所有邮件", "重要邮件":
		return true
	}
	return false
}

// dialIMAP connects over TLS and logs in. The caller must Close the client.
// A watchdog goroutine closes the connection as soon as ctx is done, so any
// in-flight IMAP command (LOGIN/LIST/FETCH wait on channels that otherwise
// never time out) unblocks with an error instead of hanging the caller.
func dialIMAP(ctx context.Context, cfg IMAPConfig) (*imapclient.Client, error) {
	host := strings.TrimSpace(cfg.Host)
	if host == "" || strings.TrimSpace(cfg.Username) == "" || cfg.Password == "" {
		return nil, errors.New("IMAP 配置不完整：需要服务器地址、账号与授权码")
	}
	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		return nil, errors.New("服务器地址应为主机名（如 imap.163.com），不要带协议前缀")
	}
	conn, err := dialIMAPAddr(ctx, imapAddress(cfg))
	if err != nil {
		return nil, fmt.Errorf("连接 IMAP 服务器失败：%v", err)
	}
	tlsConfig := &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
	tlsConn := tls.Client(conn, tlsConfig)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("TLS 握手失败：%v", err)
	}
	client := imapclient.New(tlsConn, &imapclient.Options{
		TLSConfig: tlsConfig,
		WordDecoder: &mime.WordDecoder{
			CharsetReader: func(charsetLabel string, input io.Reader) (io.Reader, error) {
				return charset.NewReaderLabel(charsetLabel, input)
			},
		},
	})
	// Watchdog: closing the socket cancels every pending command. The caller
	// releases it via the returned cleanup (deferred together with Close).
	watchCtx, watchCancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		select {
		case <-watchCtx.Done():
			_ = client.Close()
		case <-done:
		}
	}()
	cleanup := func() {
		close(done)
		watchCancel()
	}
	// 必须等完服务器问候再登录：QQ 邮箱（以及部分国内服务商）会丢弃在
	// greeting 之前流水线发送的 LOGIN，表现为发完命令后没有任何响应直到超时。
	if err := client.WaitGreeting(); err != nil {
		cleanup()
		_ = client.Close()
		return nil, fmt.Errorf("IMAP 服务器问候异常：%v", err)
	}
	if err := client.Login(strings.TrimSpace(cfg.Username), cfg.Password).Wait(); err != nil {
		cleanup()
		_ = client.Close()
		if ctx.Err() != nil {
			return nil, fmt.Errorf("连接超时：请检查服务器地址、端口与网络（%v）", err)
		}
		return nil, fmt.Errorf("登录失败：检查账号与客户端授权码（%v）", err)
	}
	// Attach cleanup to the client's lifetime for the caller's convenience.
	go func() {
		select {
		case <-client.Closed():
			cleanup()
		case <-done:
		}
	}()
	return client, nil
}

// imapContext bounds one IMAP round-trip (dial + login + commands). IMAP
// servers that accept TCP but never answer must not wedge the scheduler or
// the settings test endpoint.
func imapContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, timeout)
}

func normalizeEmailSyncState(username string, state EmailSyncState) EmailSyncState {
	username = strings.TrimSpace(username)
	result := EmailSyncState{Username: username, Folders: make(map[string]EmailFolderState)}
	if strings.EqualFold(strings.TrimSpace(state.Username), username) {
		for folder, cursor := range state.Folders {
			result.Folders[folder] = cursor
		}
	}
	return result
}

// FetchNewEmails mirrors all syncable folders. Each folder is advanced by UID
// (keyed by UIDVALIDITY), never by \Seen, and BODY.PEEK is used so the user's
// mailbox read state is untouched.
func FetchNewEmails(ctx context.Context, cfg IMAPConfig, state EmailSyncState, limit int) (EmailSyncResult, error) {
	if limit <= 0 || limit > FetchEmailBatchLimit {
		limit = FetchEmailBatchLimit
	}
	result := EmailSyncResult{State: normalizeEmailSyncState(cfg.Username, state)}
	if result.State.Folders == nil {
		result.State.Folders = make(map[string]EmailFolderState)
	}

	ctx, cancel := imapContext(ctx, imapFetchTimeout)
	defer cancel()
	client, err := dialIMAP(ctx, cfg)
	if err != nil {
		return result, err
	}
	defer func() { _ = client.Close() }()

	items, err := client.List("", "*", nil).Collect()
	if err != nil {
		return result, fmt.Errorf("获取文件夹列表失败：%v", err)
	}
	folders := syncableMailboxes(items)
	if len(folders) == 0 {
		folders = []*imap.ListData{{Mailbox: "INBOX"}}
	}

	remaining := limit
	for index, folder := range folders {
		if remaining <= 0 {
			break
		}
		// Fair share: one busy folder must not starve the rest.
		remainingFolders := len(folders) - index
		perFolder := remaining / remainingFolders
		if perFolder > EmailPerFolderLimit {
			perFolder = EmailPerFolderLimit
		}
		if perFolder < 1 {
			perFolder = 1
		}
		if perFolder > remaining {
			perFolder = remaining
		}
		emails, updated, warning := fetchFolderNewEmails(client, folder.Mailbox, perFolder, result.State.Folders[folder.Mailbox])
		if warning != nil {
			result.Warnings = append(result.Warnings, warning)
			continue
		}
		result.Emails = append(result.Emails, emails...)
		result.State.Folders[folder.Mailbox] = updated
		result.Folders++
		remaining -= len(emails)
	}
	return result, nil
}

// fetchFolderNewEmails selects one folder and returns up to limit new messages
// plus the advanced UID cursor. On a first/UIDVALIDITY-changed visit it imports
// the latest limit messages and baselines older history; afterwards it walks
// forward from the stored UID so no message is missed even when already read in
// another mail client.
func fetchFolderNewEmails(client *imapclient.Client, mailbox string, limit int, previous EmailFolderState) ([]FetchedEmail, EmailFolderState, error) {
	name, data, err := selectMailboxData(client, mailbox)
	if err != nil {
		return nil, previous, fmt.Errorf("打开邮箱文件夹 %s 失败：%v", mailbox, err)
	}
	next := EmailFolderState{UIDValidity: data.UIDValidity, LastUID: previous.LastUID}
	if previous.UIDValidity != data.UIDValidity {
		// UIDVALIDITY changed: old UIDs are meaningless.
		next.LastUID = 0
	}
	if next.LastUID == 0 {
		// First sync: import the newest messages, then baseline the rest.
		if data.NumMessages == 0 {
			if data.UIDNext > 0 {
				next.LastUID = uint32(data.UIDNext) - 1
			}
			return nil, next, nil
		}
		count := uint32(limit)
		if count > data.NumMessages {
			count = data.NumMessages
		}
		start := data.NumMessages - count + 1
		seq := imap.SeqSet{}
		seq.AddRange(start, 0)
		emails, err := fetchEmailBodies(client, seq, name)
		if err != nil {
			return nil, previous, fmt.Errorf("拉取邮件正文失败：%v", err)
		}
		next.LastUID = maxFetchedUID(emails, next.LastUID)
		if next.LastUID == 0 && data.UIDNext > 0 {
			next.LastUID = uint32(data.UIDNext) - 1
		}
		return emails, next, nil
	}

	uidSet := imap.UIDSet{}
	uidSet.AddRange(imap.UID(next.LastUID+1), 0)
	search, err := client.UIDSearch(&imap.SearchCriteria{UID: []imap.UIDSet{uidSet}}, nil).Wait()
	if err != nil {
		return nil, previous, fmt.Errorf("检索新邮件失败：%v", err)
	}
	var uids []imap.UID
	if set, ok := search.All.(imap.UIDSet); ok {
		uids, _ = set.Nums()
	}
	if len(uids) == 0 {
		if data.UIDNext > 0 {
			next.LastUID = uint32(data.UIDNext) - 1
		}
		return nil, next, nil
	}
	// Oldest first, so a large backlog is drained over successive polls
	// without skipping messages.
	if len(uids) > limit {
		uids = uids[:limit]
	}
	emails, err := fetchEmailBodies(client, imap.UIDSetNum(uids...), name)
	if err != nil {
		return nil, previous, fmt.Errorf("拉取邮件正文失败：%v", err)
	}
	next.LastUID = maxFetchedUID(emails, next.LastUID)
	return emails, next, nil
}

func fetchEmailBodies(client *imapclient.Client, numSet imap.NumSet, mailbox string) ([]FetchedEmail, error) {
	// 不取 ENVELOPE：部分国内服务商（QQ 实测）返回的 ENVELOPE 不符合库的
	// 解析假设（in envelope: expected ')'），而正文里已有全部信头，自己解析。
	cmd := client.Fetch(numSet, &imap.FetchOptions{
		UID:         true,
		BodySection: []*imap.FetchItemBodySection{{Peek: true}},
	})
	buffers, err := cmd.Collect()
	if err != nil {
		return nil, err
	}
	result := make([]FetchedEmail, 0, len(buffers))
	for _, buffer := range buffers {
		email := FetchedEmail{UID: uint32(buffer.UID), Mailbox: mailbox}
		for _, section := range buffer.BodySection {
			if section.Section != nil && section.Section.Specifier == imap.PartSpecifierNone {
				email.Raw = section.Bytes
				break
			}
		}
		if len(email.Raw) == 0 {
			continue
		}
		result = append(result, email)
	}
	return result, nil
}

func maxFetchedUID(emails []FetchedEmail, fallback uint32) uint32 {
	max := fallback
	for _, email := range emails {
		if email.UID > max {
			max = email.UID
		}
	}
	return max
}

// ListMailboxes returns mailbox names for the settings "test connection"
// endpoint.
func ListMailboxes(ctx context.Context, cfg IMAPConfig) ([]string, error) {
	ctx, cancel := imapContext(ctx, imapListTimeout)
	defer cancel()
	client, err := dialIMAP(ctx, cfg)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()
	items, err := client.List("", "*", nil).Collect()
	if err != nil {
		return nil, fmt.Errorf("获取文件夹列表失败：%v", err)
	}
	names := mailboxNames(items)
	sort.Strings(names)
	return names, nil
}
