package shared

/*
#include <stdbool.h>
*/
import "C"

func setCString(cchar **C.char, str *string) {
	if str != nil {
		*cchar = C.CString(*str)
	}
}

func setCInt(cint *C.int, i *int) {
	if i != nil {
		*cint = C.int(*i)
	}
}

func setCLong(clong *C.longlong, l *int64) {
	if l != nil {
		*clong = C.longlong(*l)
	}
}

func setCBool(cbool *C.bool, b *bool) {
	if b != nil {
		*cbool = C.bool(*b)
	}
}
