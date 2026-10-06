package grid

const third = 1.0 / 3.0

// presets são as posições fixas da tela, como fração da área útil.
var presets = map[string]Frac{
	"left":   {0, 0, 0.5, 1},
	"right":  {0.5, 0, 0.5, 1},
	"top":    {0, 0, 1, 0.5},
	"bottom": {0, 0.5, 1, 0.5},

	"top-left":     {0, 0, 0.5, 0.5},
	"top-right":    {0.5, 0, 0.5, 0.5},
	"bottom-left":  {0, 0.5, 0.5, 0.5},
	"bottom-right": {0.5, 0.5, 0.5, 0.5},

	"left-third":       {0, 0, third, 1},
	"center-third":     {third, 0, third, 1},
	"right-third":      {2 * third, 0, third, 1},
	"left-two-thirds":  {0, 0, 2 * third, 1},
	"right-two-thirds": {third, 0, 2 * third, 1},

	"maximize": {0, 0, 1, 1},
	"center":   {0.15, 0.1, 0.7, 0.8},
}

// Preset devolve a área de um preset pelo nome.
func Preset(name string) (Frac, bool) {
	f, ok := presets[name]
	return f, ok
}

// PresetNames lista os nomes de todos os presets.
func PresetNames() []string {
	out := make([]string, 0, len(presets))
	for name := range presets {
		out = append(out, name)
	}
	return out
}
