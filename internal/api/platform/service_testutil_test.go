package platform

import (
	"sync/atomic"
)

// Split from service_test.go; test bodies moved verbatim.

// platformTestDBSeq 给每次调用发互不重名的内存库（-count=3 残留行会撞 UNIQUE）。
var platformTestDBSeq atomic.Int64
