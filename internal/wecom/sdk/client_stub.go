//go:build !linux || !amd64 || !cgo

package sdk

func newNativeClient(_, _ string) (Client, error) {
	return nil, ErrUnavailable
}
