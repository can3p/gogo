package lorem

import "testing"

func TestHasLorem(t *testing.T) {
	var examples = []struct {
		phrase     string
		maxMatches int
		hasLorem   bool
	}{
		{
			phrase:     "lorem is cool",
			maxMatches: 0,
			hasLorem:   true,
		},
		{
			phrase:     "lorem is IPSUM",
			maxMatches: 1,
			hasLorem:   true,
		},
		{
			phrase:     "lorem is IPSUM",
			maxMatches: 2,
			hasLorem:   true,
		},
		{
			phrase:     "lorem is IPSUM",
			maxMatches: 3,
			hasLorem:   false,
		},
	}

	for idx, ex := range examples {
		hasLorem := HasLorem(ex.phrase, ex.maxMatches)

		if hasLorem != ex.hasLorem {
			t.Errorf("Example [%d]: wanted [%v], got [%v]", idx+1, ex.hasLorem, hasLorem)
		}
	}
}
