package lex

// Tokenize scans src into a token stream that includes whitespace and comment
// trivia, terminated by a single EOF token. The concatenation of every token's
// Text equals src. Tokenize never panics; malformed input yields a diagnostic and
// a best-effort token so scanning can continue.
func Tokenize(src string) ([]Token, []Diagnostic) {
	s := &scanner{src: src, line: 1, col: 1}
	s.run()
	return s.toks, s.diags
}

type scanner struct {
	src    string
	offset int
	line   int
	col    int
	interp []int // brace-depth stack; len>0 => inside an interpolation expression
	toks   []Token
	diags  []Diagnostic
}

func (s *scanner) atEnd() bool { return s.offset >= len(s.src) }

func (s *scanner) peek() byte {
	if s.atEnd() {
		return 0
	}
	return s.src[s.offset]
}

func (s *scanner) peekAt(n int) byte {
	if i := s.offset + n; i >= 0 && i < len(s.src) {
		return s.src[i]
	}
	return 0
}

func (s *scanner) pos() Pos { return Pos{s.offset, s.line, s.col} }

func (s *scanner) advance() byte {
	c := s.src[s.offset]
	s.offset++
	if c == '\n' {
		s.line++
		s.col = 1
	} else {
		s.col++
	}
	return c
}

func (s *scanner) emit(kind Kind, start Pos) {
	s.toks = append(s.toks, Token{Kind: kind, Text: s.src[start.Offset:s.offset], Start: start, End: s.pos()})
}

func (s *scanner) errAt(p Pos, msg string) {
	s.diags = append(s.diags, Diagnostic{Pos: p, Message: msg})
}

func (s *scanner) run() {
	for !s.atEnd() {
		c := s.peek()
		switch {
		case isSpace(c):
			s.scanWhitespace()
		case c == '-' && s.peekAt(1) == '-':
			s.scanComment()
		case isNameStart(c):
			s.scanName()
		case isDigit(c) || (c == '.' && isDigit(s.peekAt(1))):
			s.scanNumber()
		case c == '"' || c == '\'':
			s.scanShortString()
		case c == '[':
			if level, ok := s.longBracketLevel(); ok {
				s.scanLongString(level)
			} else {
				s.scanSymbol()
			}
		case c == '`':
			s.scanInterpStart()
		case c == '}' && len(s.interp) > 0 && s.interp[len(s.interp)-1] == 0:
			s.scanInterpResume()
		default:
			s.scanSymbol()
		}
	}
	s.toks = append(s.toks, Token{Kind: EOF, Start: s.pos(), End: s.pos()})
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\v' || c == '\f'
}
func isDigit(c byte) bool     { return c >= '0' && c <= '9' }
func isHex(c byte) bool       { return isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') }
func isNameStart(c byte) bool { return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
func isNameCont(c byte) bool  { return isNameStart(c) || isDigit(c) }

func (s *scanner) scanWhitespace() {
	start := s.pos()
	for !s.atEnd() && isSpace(s.peek()) {
		s.advance()
	}
	s.emit(Whitespace, start)
}

func (s *scanner) scanName() {
	start := s.pos()
	for !s.atEnd() && isNameCont(s.peek()) {
		s.advance()
	}
	s.emit(Name, start)
}

var symbols3 = [...]string{"...", "//=", "..="}
var symbols2 = [...]string{"==", "~=", "<=", ">=", "..", "::", "->", "+=", "-=", "*=", "/=", "%=", "^=", "//"}

func isSingleSymbol(c byte) bool {
	switch c {
	case '+', '-', '*', '/', '%', '^', '#', '<', '>', '=',
		'(', ')', '{', '}', '[', ']', ';', ':', ',', '.',
		'&', '|', '?', '@':
		return true
	}
	return false
}

func (s *scanner) scanSymbol() {
	start := s.pos()
	if s.offset+3 <= len(s.src) {
		three := s.src[s.offset : s.offset+3]
		for _, sym := range symbols3 {
			if three == sym {
				s.advance()
				s.advance()
				s.advance()
				s.emit(Symbol, start)
				return
			}
		}
	}
	if s.offset+2 <= len(s.src) {
		two := s.src[s.offset : s.offset+2]
		for _, sym := range symbols2 {
			if two == sym {
				s.advance()
				s.advance()
				s.emit(Symbol, start)
				return
			}
		}
	}
	c := s.peek()
	if isSingleSymbol(c) {
		s.advance()
		s.emit(Symbol, start)
		s.trackBrace(c)
		return
	}
	s.advance()
	s.emit(Invalid, start)
	s.errAt(start, "unexpected character")
}
