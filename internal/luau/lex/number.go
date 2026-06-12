package lex

// scanNumber consumes a numeric lexeme. It is permissive: it captures the run of
// number characters (the parser validates with luau.IsValidNumberLiteral). It
// never consumes a ".." (concatenation) as part of the number.
func (s *scanner) scanNumber() {
	start := s.pos()
	if s.peek() == '0' && (s.peekAt(1) == 'x' || s.peekAt(1) == 'X') {
		s.advance()
		s.advance()
		s.consumeWhile(func(c byte) bool { return isHex(c) || c == '_' })
		s.consumeFractionAndExponent(isHex, 'p', 'P')
	} else if s.peek() == '0' && (s.peekAt(1) == 'b' || s.peekAt(1) == 'B') {
		s.advance()
		s.advance()
		s.consumeWhile(func(c byte) bool { return c == '0' || c == '1' || c == '_' })
	} else {
		s.consumeWhile(func(c byte) bool { return isDigit(c) || c == '_' })
		s.consumeFractionAndExponent(isDigit, 'e', 'E')
	}
	s.emit(Number, start)
}

func (s *scanner) consumeWhile(pred func(byte) bool) {
	for !s.atEnd() && pred(s.peek()) {
		s.advance()
	}
}

// consumeFractionAndExponent consumes an optional single '.' fraction (guarded so a
// ".." is never swallowed) and an optional exponent introduced by e1/e2.
func (s *scanner) consumeFractionAndExponent(digit func(byte) bool, e1, e2 byte) {
	if !s.atEnd() && s.peek() == '.' && s.peekAt(1) != '.' {
		s.advance()
		s.consumeWhile(func(c byte) bool { return digit(c) || c == '_' })
	}
	if !s.atEnd() && (s.peek() == e1 || s.peek() == e2) {
		s.advance()
		if s.peek() == '+' || s.peek() == '-' {
			s.advance()
		}
		s.consumeWhile(func(c byte) bool { return isDigit(c) || c == '_' })
	}
}
