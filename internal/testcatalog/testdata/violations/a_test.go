package fixture

const bucket = "b"

type helperState struct{}

func helper() {}

func (s *RunSuite) startThing() {}

func (s *RunSuite) TestRuns() {}

func (s *NobodySuite) TestNeverRuns() {}
