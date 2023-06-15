package logging

import "github.com/kr/pretty"

func LogValue[A any](a A, t ...any) A {
	vals := []any{}
	vals = append(vals, t...)
	vals = append(vals, a)
	pretty.Println(vals...)

	return a
}
