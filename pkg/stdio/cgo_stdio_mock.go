//go:build ci

package stdio

type CgoStdioMock struct {
}

func init() {
	Stderr = &CgoStdioMock{}
}

var _ CgoStdio = (*CgoStdioMock)(nil)

// Capture mocks the original call by returning an empty string.
func (c *CgoStdioMock) Capture(f func()) string {
	f()
	return ""
}
