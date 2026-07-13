//go:build linux && amd64 && cgo

package sdk

import "testing"

func TestEmbeddedSDKLoads(t *testing.T) {
	if err := loadEmbeddedSDK(); err != nil {
		t.Fatalf("load embedded SDK: %v", err)
	}
}
