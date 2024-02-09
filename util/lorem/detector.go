package lorem

import (
	"regexp"
	"strings"
)

var lorem = func() map[string]struct{} {
	out := map[string]struct{}{}
	s := `lorem ipsum dolor amet consectetur adipiscing elit eiusmod tempor incididunt labore dolore magna aliqua enim minim veniam quis nostrud exercitation ullamco laboris nisi aliquip commodo consequat duis aute irure dolor reprehenderit voluptate velit esse cillum dolore fugiat nulla pariatur excepteur sint occaecat cupidatat proident sunt culpa officia deserunt mollit anim laborum`

	for _, w := range strings.Split(s, " ") {
		out[w] = struct{}{}
	}

	return out
}()

var re = regexp.MustCompile(`[\s,.]+`)

func HasLorem(s string, maxMatches int) bool {
	t := re.Split(strings.ToLower(s), -1)

	for _, t := range t {
		if _, ok := lorem[t]; ok {
			maxMatches--

			if maxMatches <= 0 {
				return true
			}
		}
	}

	return false
}
