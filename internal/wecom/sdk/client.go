package sdk

import (
	"context"
	"errors"
	"time"
)

var ErrUnavailable = errors.New("wecom archive SDK is only available in linux/amd64 CGO builds")

type EncryptedRecord struct {
	Seq              uint64 `json:"seq"`
	MsgID            string `json:"msgid"`
	PublicKeyVersion uint32 `json:"publickey_ver"`
	EncryptedKey     string `json:"encrypt_random_key"`
	EncryptedMessage string `json:"encrypt_chat_msg"`
}

type PlainMessage struct {
	MsgID   string   `json:"msgid"`
	Action  string   `json:"action"`
	From    string   `json:"from"`
	ToList  []string `json:"tolist"`
	RoomID  string   `json:"roomid"`
	MsgTime int64    `json:"msgtime"`
	MsgType string   `json:"msgtype"`
	Text    struct {
		Content string `json:"content"`
	} `json:"text"`
}

func (m PlainMessage) Time() time.Time {
	return time.UnixMilli(m.MsgTime)
}

type Client interface {
	Pull(ctx context.Context, seq uint64, limit uint32) ([]EncryptedRecord, error)
	Decrypt(ctx context.Context, record EncryptedRecord, privateKeyPEM string) (*PlainMessage, error)
	Close() error
}

type Factory interface {
	New(corpID, secret string) (Client, error)
}

type NativeFactory struct{}

func (NativeFactory) New(corpID, secret string) (Client, error) {
	return newNativeClient(corpID, secret)
}
