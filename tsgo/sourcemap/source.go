package sourcemap

import "rotor/tsgo/core"

type Source interface {
	Text() string
	FileName() string
	ECMALineMap() []core.TextPos
}
