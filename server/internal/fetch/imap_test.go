package fetch

import (
	"testing"

	"github.com/emersion/go-imap/v2"
)

func TestResolveMailboxName(t *testing.T) {
	items := []*imap.ListData{
		{Mailbox: "INBOX"},
		{Delim: '/', Mailbox: "其他文件夹"},
		{Delim: '/', Mailbox: "其他文件夹/Email"},
		{Delim: '/', Mailbox: "其他文件夹/Archive"},
		{Delim: '/', Mailbox: "其他文件夹/Email/2025"},
	}

	// 完整路径直接命中（大小写不敏感）。
	if got, _ := resolveMailboxName("other/email", items); got != "" {
		t.Fatalf("unexpected cross-separator match: %q", got)
	}
	if got, _ := resolveMailboxName("其他文件夹/email", items); got != "其他文件夹/Email" {
		t.Fatalf("exact path match = %q", got)
	}

	// 用户只填叶子名：唯一匹配时使用服务器返回的完整路径。
	if got, _ := resolveMailboxName("Email", items); got != "其他文件夹/Email" {
		t.Fatalf("unique leaf match = %q", got)
	}

	// 多个同名叶子必须拒绝，不能猜。
	ambiguous := append(items, &imap.ListData{Delim: '/', Mailbox: "INBOX/Email"})
	if got, candidates := resolveMailboxName("Email", ambiguous); got != "" || len(candidates) != 2 {
		t.Fatalf("ambiguous leaf should not resolve: %q %v", got, candidates)
	}
}

func TestResolveMailboxNameSkipsNoSelect(t *testing.T) {
	items := []*imap.ListData{
		{Delim: '/', Mailbox: "归档", Attrs: []imap.MailboxAttr{imap.MailboxAttrNoSelect}},
		{Delim: '/', Mailbox: "其他文件夹/Email"},
	}
	if got, _ := resolveMailboxName("Email", items); got != "其他文件夹/Email" {
		t.Fatalf("leaf match = %q", got)
	}
}

func TestSyncableMailboxesExcludesSystemFolders(t *testing.T) {
	items := []*imap.ListData{
		{Delim: '/', Mailbox: "INBOX"},
		{Delim: '/', Mailbox: "其他文件夹/Email"},
		{Delim: '/', Mailbox: "Sent", Attrs: []imap.MailboxAttr{imap.MailboxAttrSent}},
		{Delim: '/', Mailbox: "已发送"},
		{Delim: '/', Mailbox: "Drafts", Attrs: []imap.MailboxAttr{imap.MailboxAttrDrafts}},
		{Delim: '/', Mailbox: "Trash", Attrs: []imap.MailboxAttr{imap.MailboxAttrTrash}},
		{Delim: '/', Mailbox: "Junk", Attrs: []imap.MailboxAttr{imap.MailboxAttrJunk}},
		{Delim: '/', Mailbox: "All Mail", Attrs: []imap.MailboxAttr{imap.MailboxAttrAll}},
		{Delim: '/', Mailbox: "Parent", Attrs: []imap.MailboxAttr{imap.MailboxAttrNoSelect}},
	}
	got := syncableMailboxes(items)
	if len(got) != 2 || got[0].Mailbox != "INBOX" || got[1].Mailbox != "其他文件夹/Email" {
		t.Fatalf("syncable folders = %+v", got)
	}
}

func TestEmailSyncStateResetsOnAccountChange(t *testing.T) {
	state := EmailSyncState{
		Username: "old@example.com",
		Folders: map[string]EmailFolderState{
			"INBOX": {UIDValidity: 1, LastUID: 99},
		},
	}
	got := normalizeEmailSyncState("new@example.com", state)
	if got.Username != "new@example.com" || len(got.Folders) != 0 {
		t.Fatalf("old account state leaked: %+v", got)
	}
	same := normalizeEmailSyncState("OLD@example.com", state)
	if len(same.Folders) != 1 || same.Folders["INBOX"].LastUID != 99 {
		t.Fatalf("same account state should be kept: %+v", same)
	}
}
