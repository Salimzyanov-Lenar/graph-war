// mapgen-preview renders generated maps into an HTML file with inline
// SVG so humans can review map quality (experiment D02) without
// running the game. Usage:
//
//	go run ./cmd/mapgen-preview [firstSeed lastSeed] > maps.html
package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/Salimzyanov-Lenar/graph-war/backend/internal/game"
)

const scale = 6 // px per world unit

func main() {
	first, last := uint64(1), uint64(12)
	if len(os.Args) == 3 {
		a, err1 := strconv.ParseUint(os.Args[1], 10, 64)
		b, err2 := strconv.ParseUint(os.Args[2], 10, 64)
		if err1 == nil && err2 == nil {
			first, last = a, b
		}
	}

	fmt.Printf(`<!doctype html>
<html><head><meta charset="utf-8"><title>Graphwar: карты D02</title>
<style>
body{font-family:system-ui,sans-serif;background:#111;color:#eee;margin:16px}
.grid{display:flex;flex-wrap:wrap;gap:16px}
.card{background:#1b1b1b;border:1px solid #333;border-radius:8px;padding:8px}
.card p{margin:4px 0 0;font-size:12px;color:#aaa}
svg{display:block;background:#0d1117}
</style></head><body>
<h1>Эксперимент D02: случайные карты</h1><div class="grid">
`)

	params := game.DefaultMapParams()
	fallbackShown := false
	for seed := first; seed <= last; seed++ {
		m := game.GenerateMap(seed, params)
		if m.Obstacles[0].X == 50 && m.Obstacles[0].R == 6 && len(m.Obstacles) == 3 {
			fallbackShown = true
		}
		fmt.Printf(`<div class="card">%s<p>seed %d · препятствий: %d</p></div>`+"\n",
			svgMap(m), seed, len(m.Obstacles))
	}
	if fallbackShown {
		fmt.Println(`<p style="color:#e66">Внимание: среди карт встретился fallback — параметрам тесно.</p>`)
	}
	fmt.Println("</div></body></html>")
}

func svgMap(m game.GameMap) string {
	w := int(game.FieldWidth * scale)
	h := int(game.FieldHeight * scale)
	s := fmt.Sprintf(`<svg width="%d" height="%d" viewBox="0 0 %d %d">`, w, h, w, h)
	for _, o := range m.Obstacles {
		s += fmt.Sprintf(`<circle cx="%.0f" cy="%.0f" r="%.0f" fill="#3b4252" stroke="#88c0d0" stroke-width="2"/>`,
			o.X*scale, o.Y*scale, o.R*scale)
	}
	for _, f := range m.Fighters {
		color := "#a3be8c"
		if f.Side == game.SideB {
			color = "#bf616a"
		}
		s += fmt.Sprintf(`<circle cx="%.0f" cy="%.0f" r="%.0f" fill="%s"/>`,
			f.X*scale, f.Y*scale, game.FighterRadius*scale, color)
	}
	return s + "</svg>"
}
