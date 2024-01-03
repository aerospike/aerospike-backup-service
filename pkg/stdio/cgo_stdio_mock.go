//go:build ci

package stdio

type CgoStdio struct {
}

// Capture mocks the original call by returning an empty string.
func (c *CgoStdio) Capture(f func()) string {
	f()
	return ""
}
