//go:build linux && amd64 && cgo

package sdk

/*
#cgo LDFLAGS: -ldl
#include <stdlib.h>
#include <dlfcn.h>

typedef void WeWorkFinanceSdk_t;
typedef void Slice_t;
typedef WeWorkFinanceSdk_t* (*fnNewSdk)(void);
typedef int (*fnInit)(WeWorkFinanceSdk_t*, const char*, const char*);
typedef int (*fnGetChatData)(WeWorkFinanceSdk_t*, unsigned long long, unsigned int, const char*, const char*, int, Slice_t*);
typedef int (*fnDecryptData)(const char*, const char*, Slice_t*);
typedef void (*fnDestroySdk)(WeWorkFinanceSdk_t*);
typedef Slice_t* (*fnNewSlice)(void);
typedef void (*fnFreeSlice)(Slice_t*);
typedef char* (*fnGetContentFromSlice)(Slice_t*);
typedef int (*fnGetSliceLen)(Slice_t*);

static void *sdk_lib;
static fnNewSdk pNewSdk;
static fnInit pInit;
static fnGetChatData pGetChatData;
static fnDecryptData pDecryptData;
static fnDestroySdk pDestroySdk;
static fnNewSlice pNewSlice;
static fnFreeSlice pFreeSlice;
static fnGetContentFromSlice pGetContentFromSlice;
static fnGetSliceLen pGetSliceLen;

static int load_sdk(const char *path) {
  if (sdk_lib) return 0;
  sdk_lib = dlopen(path, RTLD_NOW | RTLD_LOCAL);
  if (!sdk_lib) return -1;
  pNewSdk = (fnNewSdk)dlsym(sdk_lib, "NewSdk");
  pInit = (fnInit)dlsym(sdk_lib, "Init");
  pGetChatData = (fnGetChatData)dlsym(sdk_lib, "GetChatData");
  pDecryptData = (fnDecryptData)dlsym(sdk_lib, "DecryptData");
  pDestroySdk = (fnDestroySdk)dlsym(sdk_lib, "DestroySdk");
  pNewSlice = (fnNewSlice)dlsym(sdk_lib, "NewSlice");
  pFreeSlice = (fnFreeSlice)dlsym(sdk_lib, "FreeSlice");
  pGetContentFromSlice = (fnGetContentFromSlice)dlsym(sdk_lib, "GetContentFromSlice");
  pGetSliceLen = (fnGetSliceLen)dlsym(sdk_lib, "GetSliceLen");
  if (!pNewSdk || !pInit || !pGetChatData || !pDecryptData || !pDestroySdk ||
      !pNewSlice || !pFreeSlice || !pGetContentFromSlice || !pGetSliceLen) return -2;
  return 0;
}
static const char* sdk_error(void) { const char *e = dlerror(); return e ? e : "unknown dlopen error"; }
static WeWorkFinanceSdk_t* sdk_new(void) { return pNewSdk(); }
static int sdk_init(WeWorkFinanceSdk_t *s, const char *corp, const char *secret) { return pInit(s, corp, secret); }
static void sdk_destroy(WeWorkFinanceSdk_t *s) { if (s) pDestroySdk(s); }
static Slice_t* sdk_slice_new(void) { return pNewSlice(); }
static void sdk_slice_free(Slice_t *s) { if (s) pFreeSlice(s); }
static int sdk_slice_len(Slice_t *s) { return pGetSliceLen(s); }
static char* sdk_slice_content(Slice_t *s) { return pGetContentFromSlice(s); }
static int sdk_pull(WeWorkFinanceSdk_t *s, unsigned long long seq, unsigned int limit, Slice_t *out) {
  return pGetChatData(s, seq, limit, "", "", 10000, out);
}
static int sdk_decrypt(const char *key, const char *msg, Slice_t *out) { return pDecryptData(key, msg, out); }
*/
import "C"

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"unsafe"
)

//go:embed native/linux_amd64/libWeWorkFinanceSdk_C.so
var nativeFiles embed.FS

var loadOnce sync.Once
var loadErr error

type nativeClient struct {
	mu     sync.Mutex
	handle unsafe.Pointer
}

func newNativeClient(corpID, secret string) (Client, error) {
	loadOnce.Do(func() { loadErr = loadEmbeddedSDK() })
	if loadErr != nil {
		return nil, loadErr
	}
	corp := C.CString(corpID)
	sec := C.CString(secret)
	defer C.free(unsafe.Pointer(corp))
	defer C.free(unsafe.Pointer(sec))
	h := C.sdk_new()
	if h == nil {
		return nil, fmt.Errorf("wecom sdk NewSdk returned nil")
	}
	if code := int(C.sdk_init(h, corp, sec)); code != 0 {
		C.sdk_destroy(h)
		return nil, fmt.Errorf("wecom sdk Init failed: code=%d", code)
	}
	return &nativeClient{handle: h}, nil
}

func loadEmbeddedSDK() error {
	b, err := nativeFiles.ReadFile("native/linux_amd64/libWeWorkFinanceSdk_C.so")
	if err != nil {
		return fmt.Errorf("read embedded wecom sdk: %w", err)
	}
	sum := sha256.Sum256(b)
	dir := filepath.Join(os.TempDir(), "agent-customer-service", "wecom-sdk")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	path := filepath.Join(dir, "libWeWorkFinanceSdk_C-"+hex.EncodeToString(sum[:8])+".so")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		tmp := path + ".tmp"
		if err := os.WriteFile(tmp, b, 0o500); err != nil {
			return err
		}
		if err := os.Rename(tmp, path); err != nil {
			return err
		}
	}
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	if code := int(C.load_sdk(cpath)); code != 0 {
		return fmt.Errorf("load embedded wecom sdk: code=%d: %s", code, C.GoString(C.sdk_error()))
	}
	return nil
}

func (c *nativeClient) Pull(ctx context.Context, seq uint64, limit uint32) ([]EncryptedRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	slice := C.sdk_slice_new()
	if slice == nil {
		return nil, fmt.Errorf("wecom sdk NewSlice returned nil")
	}
	defer C.sdk_slice_free(slice)
	if code := int(C.sdk_pull(c.handle, C.ulonglong(seq), C.uint(limit), slice)); code != 0 {
		return nil, fmt.Errorf("wecom sdk GetChatData failed: code=%d", code)
	}
	raw := C.GoBytes(unsafe.Pointer(C.sdk_slice_content(slice)), C.int(C.sdk_slice_len(slice)))
	var payload struct {
		ErrCode  int               `json:"errcode"`
		ErrMsg   string            `json:"errmsg"`
		ChatData []EncryptedRecord `json:"chatdata"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode GetChatData response: %w", err)
	}
	if payload.ErrCode != 0 {
		return nil, fmt.Errorf("wecom GetChatData response: code=%d message=%s", payload.ErrCode, payload.ErrMsg)
	}
	return payload.ChatData, nil
}

func (c *nativeClient) Decrypt(ctx context.Context, record EncryptedRecord, privateKeyPEM string) (*PlainMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	randomKey, err := decryptRandomKey(record.EncryptedKey, privateKeyPEM)
	if err != nil {
		return nil, err
	}
	key := C.CString(string(randomKey))
	msg := C.CString(record.EncryptedMessage)
	defer C.free(unsafe.Pointer(key))
	defer C.free(unsafe.Pointer(msg))

	c.mu.Lock()
	defer c.mu.Unlock()
	slice := C.sdk_slice_new()
	if slice == nil {
		return nil, fmt.Errorf("wecom sdk NewSlice returned nil")
	}
	defer C.sdk_slice_free(slice)
	if code := int(C.sdk_decrypt(key, msg, slice)); code != 0 {
		return nil, fmt.Errorf("wecom sdk DecryptData failed: code=%d", code)
	}
	raw := C.GoBytes(unsafe.Pointer(C.sdk_slice_content(slice)), C.int(C.sdk_slice_len(slice)))
	var out PlainMessage
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode decrypted message: %w", err)
	}
	return &out, nil
}

func decryptRandomKey(cipherText, privateKeyPEM string) ([]byte, error) {
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return nil, fmt.Errorf("invalid RSA private key PEM")
	}
	var key *rsa.PrivateKey
	if parsed, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		key = parsed
	} else {
		parsedAny, parseErr := x509.ParsePKCS8PrivateKey(block.Bytes)
		if parseErr != nil {
			return nil, fmt.Errorf("parse RSA private key: %w", parseErr)
		}
		var ok bool
		key, ok = parsedAny.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key is not RSA")
		}
	}
	cipherBytes, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return nil, fmt.Errorf("decode encrypted random key: %w", err)
	}
	plain, err := rsa.DecryptPKCS1v15(rand.Reader, key, cipherBytes)
	if err != nil {
		return nil, fmt.Errorf("decrypt random key: %w", err)
	}
	return plain, nil
}

func (c *nativeClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.handle != nil {
		C.sdk_destroy(c.handle)
		c.handle = nil
	}
	return nil
}
