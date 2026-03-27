package domain

// sparklineRunes maps normalized values to Unicode block characters for visual display.
var sparklineRunes = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// Sparkline converts a slice of float64 values into a sparkline string using
// Unicode block characters. Returns an empty string for empty input.
func Sparkline(values []float64) string {
	if len(values) == 0 {
		return ""
	}
	min, max := values[0], values[0]
	for _, v := range values[1:] {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	rng := max - min
	result := make([]rune, len(values))
	for i, v := range values {
		var idx int
		if rng == 0 {
			idx = len(sparklineRunes) / 2
		} else {
			idx = int((v - min) / rng * float64(len(sparklineRunes)-1))
		}
		result[i] = sparklineRunes[idx]
	}
	return string(result)
}
