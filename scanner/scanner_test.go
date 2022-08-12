package scanner

import "testing"

func BenchmarkScanLibrary(b *testing.B) {
	if err := ScanLibrary("../lib"); err != nil {
		b.Error(err)
	}
}
