module github.com/aristorinjuang/lesstruct/modules/posttypes-basic

go 1.27

toolchain go1.27.2

require (
	github.com/BurntSushi/toml v1.6.0
	github.com/aristorinjuang/lesstruct/kernel v1.0.0
	github.com/go-chi/chi/v5 v5.3.1
	github.com/stretchr/testify v1.11.1
)

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/aristorinjuang/lesstruct/kernel => ../../kernel
