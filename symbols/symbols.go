package symbols

type SymbolCode int

const (
	ScArrowUp SymbolCode = iota
	ScArrowRight
	ScArrowDown
	ScArrowLeft

	ScSpace
	ScEnter

	ScTrashCan
	ScClock
	ScPen
	ScSentMail
)

type SymbolMap map[SymbolCode]string

var NerdFontMap = SymbolMap{
	ScArrowUp:    " ",
	ScArrowRight: " ",
	ScArrowDown:  " ",
	ScArrowLeft:  " ",

	ScSpace: "󱁐 ",
	ScEnter: "↵",

	ScTrashCan: " ",
	ScClock:    " ",
	ScPen:      " ",
	ScSentMail: "󰪱 ",
}

var DefaultMap = SymbolMap{
	ScArrowUp:    "↑",
	ScArrowRight: "→",
	ScArrowDown:  "↓",
	ScArrowLeft:  "←",

	ScSpace: "␣",
	ScEnter: "↵",

	ScTrashCan: "🗑",
	ScClock:    "⏱️",
	ScPen:      "🖋️",
	ScSentMail: "📨",
}

var CurrentMap SymbolMap
