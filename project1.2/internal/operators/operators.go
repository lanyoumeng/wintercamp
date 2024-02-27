//go:build tinygo

package operators

import (
	wasilibs "github.com/corazawaf/coraza-wasilibs"
)

func Register() {
	wasilibs.RegisterRX()
	wasilibs.RegisterPM()
	wasilibs.RegisterSQLi()
	wasilibs.RegisterXSS()
}
