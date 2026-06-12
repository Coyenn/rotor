package lex

import "fmt"

type Kind uint8

const (
	EOF          Kind = iota
	Whitespace        // trivia: maximal run of space/tab/newline/CR/VT/FF
	Comment           // trivia: -- line or --[[ ]] long comment
	Name              // identifier or keyword (parser disambiguates keywords)
	Number            // numeric literal lexeme (validity checked later)
	String            // short '..'/".." or long [[..]] string
	InterpSimple      // `...` backtick string with no interpolation hole
	InterpStart       // `... {   (backtick through first {)
	InterpMid         // }...{
	InterpEnd         // }...`
	Symbol            // operator or punctuation
	Invalid           // an unrecognized character (recovered)
)

func (k Kind) String() string {
	switch k {
	case EOF:
		return "EOF"
	case Whitespace:
		return "Whitespace"
	case Comment:
		return "Comment"
	case Name:
		return "Name"
	case Number:
		return "Number"
	case String:
		return "String"
	case InterpSimple:
		return "InterpSimple"
	case InterpStart:
		return "InterpStart"
	case InterpMid:
		return "InterpMid"
	case InterpEnd:
		return "InterpEnd"
	case Symbol:
		return "Symbol"
	case Invalid:
		return "Invalid"
	default:
		return fmt.Sprintf("Kind(%d)", uint8(k))
	}
}

// Pos is a source position. Offset is a 0-based byte offset; Line and Col are
// 1-based (Col counts bytes within the current line).
type Pos struct {
	Offset int
	Line   int
	Col    int
}

type Token struct {
	Kind  Kind
	Text  string
	Start Pos
	End   Pos
}

type Diagnostic struct {
	Pos     Pos
	Message string
}
