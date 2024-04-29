package links

import "net/url"

type ArgBuilder struct {
	Args []string
}

func NewArgBuilder(params ...string) *ArgBuilder {
	return &ArgBuilder{
		Args: params,
	}
}

func (a *ArgBuilder) Shift() string {
	if len(a.Args) == 0 {
		return ""
	}

	v := a.Args[0]
	a.Args = a.Args[1:]

	return v
}

func (a *ArgBuilder) BuildQueryString() string {
	if len(a.Args) == 0 {
		return ""
	}

	params := url.Values{}

	idx := 0

	for idx < len(a.Args) {
		name := a.Args[idx]
		value := ""

		if idx+1 < len(a.Args) {
			value = a.Args[idx+1]
		}

		if _, ok := params[name]; !ok {
			params[name] = []string{value}
		} else {
			params[name] = append(params[name], value)
		}
		idx += 2
	}

	return "?" + params.Encode()
}
