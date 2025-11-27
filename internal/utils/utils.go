package utils

type StrategyType int

const (
	FullUpdate StrategyType = iota
	MinorUpdate
	PatchUpdate
)
