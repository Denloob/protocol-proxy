package symbols

type SymbolCode int

const (
	ScArrowUp SymbolCode = iota
	ScArrowRight
	ScArrowDown
	ScArrowLeft

	ScSpace
	ScEnter
)

type SymbolMap map[SymbolCode]string

var NerdFontMap = SymbolMap{
	ScArrowUp:    " ",
	ScArrowRight: " ",
	ScArrowDown:  " ",
	ScArrowLeft:  " ",

	ScSpace: "󱁐 ",
	ScEnter: "↵",
}

var DefaultMap = SymbolMap{
	ScArrowUp:    "↑",
	ScArrowRight: "→",
	ScArrowDown:  "↓",
	ScArrowLeft:  "←",

	ScSpace: "␣",
	ScEnter: "↵",
}
