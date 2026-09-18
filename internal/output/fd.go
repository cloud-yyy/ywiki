package output

import (
	"math"
	"os"
)

// FileDescriptor returns a file's descriptor as an int, reporting false when
// it does not fit. The bounds check is what keeps the uintptr-to-int
// conversion safe on every platform: descriptors are always small in practice,
// but the types do not say so.
func FileDescriptor(f *os.File) (int, bool) {
	if f == nil {
		return 0, false
	}

	fd := f.Fd()
	if fd > math.MaxInt {
		return 0, false
	}

	return int(fd), true
}
