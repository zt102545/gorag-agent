package wecom

import (
	"context"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/yourorg/agent_customer_service/internal/db"
	wecomsdk "github.com/yourorg/agent_customer_service/internal/wecom/sdk"
)

type fakeFactory struct{ client *fakeClient }

func (f fakeFactory) New(_, _ string) (wecomsdk.Client, error) { return f.client, nil }

type fakeClient struct {
	records []wecomsdk.EncryptedRecord
	plain   map[uint64]*wecomsdk.PlainMessage
}

func (f *fakeClient) Pull(_ context.Context, seq uint64, _ uint32) ([]wecomsdk.EncryptedRecord, error) {
	if seq > 0 {
		return nil, nil
	}
	return f.records, nil
}

func (f *fakeClient) Decrypt(_ context.Context, record wecomsdk.EncryptedRecord, _ string) (*wecomsdk.PlainMessage, error) {
	return f.plain[record.Seq], nil
}

func (f *fakeClient) Close() error { return nil }

func TestSyncNow_FiltersEmployeeGroupsAndText(t *testing.T) {
	gdb, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := gdb.AutoMigrate(&db.WeComArchiveConfig{}, &db.WeComMessage{}); err != nil {
		t.Fatal(err)
	}
	config := db.WeComArchiveConfig{ID: singletonConfigID, CorpID: "ww-test", Secret: "secret", PrivateKey: "private", PublicKeyVersion: 1, ArchiveUserID: "employee", GroupIDs: db.StringSlice{"room-1"}, PollIntervalSeconds: 10, AnalyzeIntervalSeconds: 600}
	if err := gdb.Create(&config).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UnixMilli()
	makeMessage := func(id, from, room, kind, content string, to []string) *wecomsdk.PlainMessage {
		msg := &wecomsdk.PlainMessage{MsgID: id, Action: "send", From: from, ToList: to, RoomID: room, MsgTime: now, MsgType: kind}
		msg.Text.Content = content
		return msg
	}
	client := &fakeClient{
		records: []wecomsdk.EncryptedRecord{{Seq: 1, MsgID: "m1", PublicKeyVersion: 1}, {Seq: 2, MsgID: "m2", PublicKeyVersion: 1}, {Seq: 3, MsgID: "m3", PublicKeyVersion: 1}, {Seq: 4, MsgID: "m4", PublicKeyVersion: 1}},
		plain: map[uint64]*wecomsdk.PlainMessage{
			1: makeMessage("m1", "customer", "room-1", "text", "怎么退款？", []string{"employee"}),
			2: makeMessage("m2", "employee", "room-1", "text", "请提供订单号", []string{"customer"}),
			3: makeMessage("m3", "customer", "room-2", "text", "不在配置群", []string{"employee"}),
			4: makeMessage("m4", "customer", "room-1", "image", "", []string{"employee"}),
		},
	}
	service := NewService(gdb, fakeFactory{client: client}, nil, nil, nil)
	result, err := service.SyncNow(context.Background())
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if result.Pulled != 4 || result.Stored != 2 || result.LastSeq != 4 {
		t.Fatalf("unexpected result: %#v", result)
	}
	var rows []db.WeComMessage
	if err := gdb.Order("seq").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Direction != "customer" || rows[1].Direction != "employee" {
		t.Fatalf("unexpected stored messages: %#v", rows)
	}
}

func TestParseJSONObject_StripsMarkdownEnvelope(t *testing.T) {
	var out extractionResult
	if err := parseJSONObject("```json\n{\"questions\":[]}\n```", &out); err != nil {
		t.Fatal(err)
	}
	if out.Questions == nil {
		t.Fatal("questions should be decoded")
	}
}

func TestTailUTF8_DoesNotSplitChineseRune(t *testing.T) {
	got := tailUTF8(strings.Repeat("中文", 100), 17)
	if !utf8.ValidString(got) {
		t.Fatalf("tail is not valid UTF-8: %q", got)
	}
	if len(got) > 17 {
		t.Fatalf("tail length = %d, want <= 17", len(got))
	}
}
