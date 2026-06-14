package diagframe

import "testing"

func TestLineColAt(t *testing.T) {
	src := "ab\ncde\nf"
	cases := []struct {
		off, wantLine, wantCol int
	}{
		{0, 1, 1},  // 'a'
		{1, 1, 2},  // 'b'
		{3, 2, 1},  // 'c'
		{6, 2, 4},  // newline after "cde" -> col past 'e'
		{7, 3, 1},  // 'f'
		{99, 3, 2}, // clamp past end
	}
	for _, c := range cases {
		line, col, _ := lineColAt(src, c.off)
		if line != c.wantLine || col != c.wantCol {
			t.Errorf("offset %d: got %d:%d, want %d:%d", c.off, line, col, c.wantLine, c.wantCol)
		}
	}
}

func TestLineTextStripsCR(t *testing.T) {
	src := "x = 1\r\ny = 2\r\n"
	got := lineText(src, 1)
	if got != "x = 1" {
		t.Errorf("lineText line 1 = %q, want %q", got, "x = 1")
	}
}
