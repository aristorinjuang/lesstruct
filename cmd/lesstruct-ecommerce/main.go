// Command lesstruct-ecommerce is the eCommerce edition: the blog module
// list plus the commerce stack (catalog products with native pricing,
// variant matrices, stock, and SKUs; token-keyed carts; first-class orders
// with checkout and the admin lifecycle surface). The blog edition is the
// root package; this binary links the commerce stack on top of it.
package main

import (
	"fmt"
	"os"

	"github.com/aristorinjuang/lesstruct/internal/app"
	commerceedition "github.com/aristorinjuang/lesstruct/internal/editions/commerce"
)

// version is the build version, stamped via -ldflags -X main.version in
// release builds (see .github/workflows/release.yml); `--version` reports it
// so an operator can confirm exactly which build is running.
var version = "dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-version") {
		fmt.Println(version)
		return
	}
	app.Run(commerceedition.Edition())
}
