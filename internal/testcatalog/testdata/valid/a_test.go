package fixture

// TestAlpha checks the first thing.
//
// A second paragraph the summary leaves out.
func (s *FirstSuite) TestAlpha() {
	s.Run("one", func() {})
	s.Run("two", func() {})
}

func (s *SecondSuite) TestBeta() {}
