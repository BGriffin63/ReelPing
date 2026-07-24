package discord

// Style is a semantic message style. Each maps to a colour and a leading symbol
// so status is conveyed by text/symbol, never colour alone.
type Style string

const (
	StyleInfo        Style = "info"
	StyleScheduled   Style = "scheduled"
	StyleMaintActive Style = "maintenance"
	StyleWarning     Style = "warning"
	StyleDegraded    Style = "degraded"
	StyleOutage      Style = "outage"
	StyleRecovery    Style = "recovery"
	StyleTest        Style = "test"
)

type styleSpec struct {
	Color  int
	Symbol string
}

var styleSpecs = map[Style]styleSpec{
	StyleInfo:        {Color: 0x3498DB, Symbol: "🔵"},
	StyleScheduled:   {Color: 0xE67E22, Symbol: "🟡"},
	StyleMaintActive: {Color: 0xE67E22, Symbol: "🟡"},
	StyleWarning:     {Color: 0xF1C40F, Symbol: "🟠"},
	StyleDegraded:    {Color: 0xF39C12, Symbol: "🟠"},
	StyleOutage:      {Color: 0xE74C3C, Symbol: "🔴"},
	StyleRecovery:    {Color: 0x2ECC71, Symbol: "🟢"},
	StyleTest:        {Color: 0x5865F2, Symbol: "🔵"},
}

func (s Style) spec() styleSpec {
	if sp, ok := styleSpecs[s]; ok {
		return sp
	}
	return styleSpecs[StyleInfo]
}

// Color returns the embed colour for the style.
func (s Style) Color() int { return s.spec().Color }

// Symbol returns the leading status symbol for the style.
func (s Style) Symbol() string { return s.spec().Symbol }
