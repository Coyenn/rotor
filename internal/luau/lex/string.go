package lex

import "strings"

func (s *scanner) scanShortString() {
	start := s.pos()
	quote := s.advance() // ' or "
	for !s.atEnd() {
		switch c := s.peek(); c {
		case '\\':
			s.advance() // backslash
			if s.atEnd() {
				break
			}
			e := s.peek()
			s.advance() // the escaped byte (covers \n, \", \\, \xHH start, etc.)
			if e == 'z' {
				// \z skips subsequent whitespace, including newlines
				s.consumeWhile(isSpace)
			}
		case quote:
			s.advance()
			s.emit(String, start)
			return
		case '\n':
			s.errAt(start, "unterminated string")
			s.emit(String, start)
			return
		default:
			s.advance()
		}
	}
	s.errAt(start, "unterminated string")
	s.emit(String, start)
}

// longBracketLevel reports whether the current offset begins a long bracket
// '[' '='* '[' and, if so, the number of '=' signs (the level).
func (s *scanner) longBracketLevel() (level int, ok bool) {
	if s.peek() != '[' {
		return 0, false
	}
	i := s.offset + 1
	for i < len(s.src) && s.src[i] == '=' {
		level++
		i++
	}
	if i < len(s.src) && s.src[i] == '[' {
		return level, true
	}
	return 0, false
}

// consumeLongBody consumes an opening long bracket of the given level and the body
// up to and including the matching close, or to EOF. Returns false if unterminated.
func (s *scanner) consumeLongBody(level int) bool {
	s.advance() // [
	for range level {
		s.advance() // =
	}
	s.advance() // [
	closing := "]" + strings.Repeat("=", level) + "]"
	for !s.atEnd() {
		if s.peek() == ']' && s.offset+len(closing) <= len(s.src) && s.src[s.offset:s.offset+len(closing)] == closing {
			for range closing {
				s.advance()
			}
			return true
		}
		s.advance()
	}
	return false
}

func (s *scanner) scanLongString(level int) {
	start := s.pos()
	if !s.consumeLongBody(level) {
		s.errAt(start, "unterminated long string")
	}
	s.emit(String, start)
}

func (s *scanner) scanComment() {
	start := s.pos()
	s.advance() // -
	s.advance() // -
	if !s.atEnd() && s.peek() == '[' {
		if level, ok := s.longBracketLevel(); ok {
			if !s.consumeLongBody(level) {
				s.errAt(start, "unterminated long comment")
			}
			s.emit(Comment, start)
			return
		}
	}
	for !s.atEnd() && s.peek() != '\n' {
		s.advance()
	}
	s.emit(Comment, start)
}
