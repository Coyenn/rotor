package printer

import (
	"rotor/tsgo/ast"
	"rotor/tsgo/tspath"
)

type SourceFileMetaDataProvider interface {
	GetSourceFileMetaData(path tspath.Path) *ast.SourceFileMetaData
}
