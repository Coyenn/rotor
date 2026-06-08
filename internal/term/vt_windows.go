package term

import (
	"io"
	"os"
	"sync"

	"golang.org/x/sys/windows"
)

// enableWindowsVT switches the console backing w into virtual-terminal mode so
// the ANSI sequences this package emits are interpreted rather than printed
// literally. Windows Terminal enables this by default, but legacy conhost does
// not; flipping ENABLE_VIRTUAL_TERMINAL_PROCESSING makes color work there too.
// It is a best-effort no-op when w is not a real console handle. Each handle is
// configured at most once.
func enableWindowsVT(w io.Writer) {
	f, ok := w.(*os.File)
	if !ok {
		return
	}
	handle := windows.Handle(f.Fd())
	vtOnce(handle).Do(func() {
		var mode uint32
		if err := windows.GetConsoleMode(handle, &mode); err != nil {
			return
		}
		_ = windows.SetConsoleMode(handle, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
	})
}

var (
	vtMu    sync.Mutex
	vtOnces = map[windows.Handle]*sync.Once{}
)

func vtOnce(h windows.Handle) *sync.Once {
	vtMu.Lock()
	defer vtMu.Unlock()
	once, ok := vtOnces[h]
	if !ok {
		once = &sync.Once{}
		vtOnces[h] = once
	}
	return once
}
