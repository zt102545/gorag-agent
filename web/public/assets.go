package widgetassets

import (
	"crypto/sha256"
	"embed"
	"fmt"
)

// Files contains the browser assets shipped inside the server binary.
//
//go:embed customer-service-widget.js
var Files embed.FS

var WidgetETag = func() string {
	data, err := Files.ReadFile("customer-service-widget.js")
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf(`"%x"`, sha256.Sum256(data))
}()
