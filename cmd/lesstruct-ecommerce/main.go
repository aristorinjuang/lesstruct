// Command lesstruct-ecommerce is the eCommerce edition: the blog module
// list plus the commerce stack (catalog products with native pricing,
// variant matrices, stock, and SKUs; token-keyed carts; first-class orders
// with checkout and the admin lifecycle surface). The blog edition is the
// root package; this binary links the commerce stack on top of it.
package main

import (
	"github.com/aristorinjuang/lesstruct/internal/app"
	commerceedition "github.com/aristorinjuang/lesstruct/internal/editions/commerce"
)

func main() {
	app.Run(commerceedition.Edition())
}
