package internal

import (
	"fmt"
	"testing"
)

func BenchmarkSprint(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fmt.Sprint(i)
	}
}
