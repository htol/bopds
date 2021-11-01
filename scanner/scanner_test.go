package scanner

import "testing"

func BenchmarkScanLibrary(b *testing.B) {
	ScanLibrary("../lib")
}
