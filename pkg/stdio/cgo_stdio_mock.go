//go:build ci

package stdio

type CgoStdio struct {
}

// NewCgoStdio returns a new CgoStdio.
func NewCgoStdio(capture bool) *CgoStdio {
	return &CgoStdio{}
}

// Stderr log capturer.
var Stderr = &CgoStdio{}

// Capture mocks the original call by returning an empty string.
func (c *CgoStdio) Capture(f func()) string {
	f()
	return ""
}
