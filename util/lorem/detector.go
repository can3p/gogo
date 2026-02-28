package lorem

import (
	"regexp"
	"strings"
)

const words = "lorem ipsum dolor amet consectetur adipiscing elit eiusmod tempor incididunt labore dolore magna aliqua enim minim veniam quis nostrud exercitation ullamco laboris nisi aliquip commodo consequat duis aute irure dolor reprehenderit voluptate velit esse cillum dolore fugiat nulla pariatur excepteur sint occaecat cupidatat proident sunt culpa officia deserunt mollit anim laborum accusamus accusantiu accusantium adipisci alias aliquam aliquid amet animi aperia aperiam architecto asperiores aspernatu aspernatur assumenda atque autem axime beatae blanditiis bore commodi cons consect consectetur consequatur consequuntur corporis corrupti culpa cum cumque cupiditate deb debitis delectus delen deleniti deserunt dicta digni dignissimos distinctio dolor dolore dolorem doloremque dolores doloribus dolorum ducimus eaque earum eiciendis eius eligendi enim etur eum even eveniet excepturi exercitationem expedit expedita explicabo facere facilis fuga fugia fugiat fugit harum illo illum impedit incidunt inventore ipsa ipsam ipsum iste itaque iure iusto labore laboriosam laborum laudantium libero lorum magnam magni maiores maxime minima minus modi molestiae molestias mollitia natus necessitatibus nemo neque nesciunt nihil nisi nobis nostrum ntium nulla numquam occaecati odio odit officia officiis omnis onsectetur optio pariatur perferendis perspiciatis piditate placeat porro praese praesentium provident ptatem quae quaerat quam quas quasi quia quibusdam quidem quis quisqua quisquam quod quos ratione recusandae reiciendis repellat repellendus reprehenderit repudiandae rerum saepe sapient sapiente sequi similique sint soluta ssimos sunt suscipit tempora tempore tempori temporibus tenetur totam ullam unde vel velit veniam veritatis vero vitae volu voluptas voluptate voluptatem voluptates voluptatibus voluptatum"

var lorem = func() map[string]struct{} {
	out := map[string]struct{}{}

	for _, w := range strings.Split(words, " ") {
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
