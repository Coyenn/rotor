package lex

// scanInterpStart handles a leading backtick. It either produces an InterpSimple
// token (no hole) or an InterpStart token (body ended at the first '{'), pushing an
// interpolation frame whose value is the current brace nesting depth (0).
func (s *scanner) scanInterpStart() {
	start := s.pos()
	s.advance() // `
	if s.scanInterpBody() {
		s.interp = append(s.interp, 0)
		s.emit(InterpStart, start)
	} else {
		s.emit(InterpSimple, start)
	}
}

// scanInterpResume handles a '}' that closes an interpolation hole (only called when
// the top interpolation frame has depth 0). It produces InterpMid (another hole
// follows) or InterpEnd (string closed), popping the frame on InterpEnd.
func (s *scanner) scanInterpResume() {
	start := s.pos()
	s.advance() // }
	if s.scanInterpBody() {
		s.emit(InterpMid, start)
	} else {
		s.interp = s.interp[:len(s.interp)-1]
		s.emit(InterpEnd, start)
	}
}

// scanInterpBody scans backtick-string content (honoring \ escapes) until an
// unescaped '`' (returns false) or an unescaped '{' (returns true). It consumes the
// terminating delimiter.
func (s *scanner) scanInterpBody() (endedAtBrace bool) {
	for !s.atEnd() {
		switch c := s.peek(); c {
		case '\\':
			s.advance()
			if !s.atEnd() {
				s.advance()
			}
		case '`':
			s.advance()
			return false
		case '{':
			s.advance()
			return true
		case '\n':
			s.errAt(s.pos(), "unterminated interpolated string")
			return false
		default:
			s.advance()
		}
	}
	s.errAt(s.pos(), "unterminated interpolated string")
	return false
}

// trackBrace updates the top interpolation frame's brace depth after a single-char
// symbol is emitted. A '}' that closes a hole (depth 0) is handled earlier by the
// run loop via scanInterpResume and never reaches here.
func (s *scanner) trackBrace(c byte) {
	if len(s.interp) == 0 {
		return
	}
	top := len(s.interp) - 1
	switch c {
	case '{':
		s.interp[top]++
	case '}':
		s.interp[top]--
	}
}
