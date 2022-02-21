module sda-filesystem

go 1.16

require (
	github.com/billziss-gh/cgofuse v1.5.0
	github.com/dgraph-io/ristretto v0.1.0
	github.com/hectane/go-acl v0.0.0-20190604041725-da78bae5fc95
	github.com/sirupsen/logrus v1.8.1
	github.com/stretchr/testify v1.7.0 // indirect
	github.com/therecipe/qt v0.0.0-20200904063919-c0c124a5770d
	golang.org/x/sys v0.0.0-20211102061401-a2f17f7b995c
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211
)

replace github.com/stretchr/testify => github.com/stretchr/testify v1.7.0
