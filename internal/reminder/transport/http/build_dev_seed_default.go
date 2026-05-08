//go:build !prod

package http

func buildAllowsDevSeed() bool {
	return true
}
