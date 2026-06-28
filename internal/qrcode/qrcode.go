package qrcode

import (
	"fmt"
	"html"
	"strings"
)

// Options controls the visual style of the generated QR SVG.
type Options struct {
	Scale      int
	Border     int
	Foreground string
	Background string
	Shape      string // classic, rounded, dots
}

// SVG encodes text as a QR Code SVG using byte mode, error correction level L,
// and QR versions 1-5. This covers the platform's public short URLs while
// keeping the project dependency-free.
func SVG(text string, scale, border int) (string, error) {
	return StyledSVG(text, Options{Scale: scale, Border: border, Foreground: "#000000", Background: "#ffffff", Shape: "classic"})
}

func StyledSVG(text string, opt Options) (string, error) {
	if opt.Scale <= 0 {
		opt.Scale = 8
	}
	if opt.Border < 0 {
		opt.Border = 4
	}
	if strings.TrimSpace(opt.Foreground) == "" {
		opt.Foreground = "#111827"
	}
	if strings.TrimSpace(opt.Background) == "" {
		opt.Background = "#ffffff"
	}
	shape := strings.ToLower(strings.TrimSpace(opt.Shape))
	if shape == "" {
		shape = "rounded"
	}
	qr, err := encode([]byte(text))
	if err != nil {
		return "", err
	}
	size := qr.size + opt.Border*2
	px := size * opt.Scale
	var b strings.Builder
	crisp := ""
	if shape == "classic" {
		crisp = ` shape-rendering="crispEdges"`
	}
	b.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d"%s>`, size, size, px, px, crisp))
	b.WriteString(fmt.Sprintf(`<rect width="100%%" height="100%%" rx="3" fill="%s"/>`, html.EscapeString(opt.Background)))
	switch shape {
	case "dots":
		b.WriteString(fmt.Sprintf(`<g fill="%s">`, html.EscapeString(opt.Foreground)))
		for y := 0; y < qr.size; y++ {
			for x := 0; x < qr.size; x++ {
				if qr.modules[y][x] {
					b.WriteString(fmt.Sprintf(`<circle cx="%g" cy="%g" r="0.43"/>`, float64(x+opt.Border)+0.5, float64(y+opt.Border)+0.5))
				}
			}
		}
		b.WriteString(`</g>`)
	case "rounded":
		b.WriteString(fmt.Sprintf(`<g fill="%s">`, html.EscapeString(opt.Foreground)))
		for y := 0; y < qr.size; y++ {
			for x := 0; x < qr.size; x++ {
				if qr.modules[y][x] {
					b.WriteString(fmt.Sprintf(`<rect x="%d" y="%d" width="1" height="1" rx="0.22"/>`, x+opt.Border, y+opt.Border))
				}
			}
		}
		b.WriteString(`</g>`)
	default:
		b.WriteString(fmt.Sprintf(`<path fill="%s" d="`, html.EscapeString(opt.Foreground)))
		for y := 0; y < qr.size; y++ {
			for x := 0; x < qr.size; x++ {
				if qr.modules[y][x] {
					b.WriteString(fmt.Sprintf("M%d %dh1v1h-1z", x+opt.Border, y+opt.Border))
				}
			}
		}
		b.WriteString(`"/>`)
	}
	b.WriteString(fmt.Sprintf(`<title>%s</title>`, html.EscapeString(text)))
	b.WriteString(`</svg>`)
	return b.String(), nil
}

type qrCode struct {
	version, size   int
	modules, isFunc [][]bool
}

var dataCodewordsL = []int{0, 19, 34, 55, 80, 108}
var eccCodewordsL = []int{0, 7, 10, 15, 20, 26}

func encode(data []byte) (*qrCode, error) {
	version := 0
	for v := 1; v <= 5; v++ {
		if 4+8+len(data)*8 <= dataCodewordsL[v]*8 {
			version = v
			break
		}
	}
	if version == 0 {
		return nil, fmt.Errorf("QR 内容过长：当前轻量编码器支持约 100 字节以内的短链接")
	}
	size := 17 + version*4
	q := &qrCode{version: version, size: size, modules: makeMatrix(size), isFunc: makeMatrix(size)}
	q.drawFunctionPatterns()
	codewords := makeDataCodewords(data, dataCodewordsL[version])
	ecc := reedSolomonRemainder(codewords, reedSolomonDivisor(eccCodewordsL[version]))
	all := append(codewords, ecc...)
	q.drawCodewords(all)
	q.applyMask0()
	q.drawFormatBits(0) // ECL L, mask 0
	return q, nil
}

func makeMatrix(size int) [][]bool {
	m := make([][]bool, size)
	for i := range m {
		m[i] = make([]bool, size)
	}
	return m
}
func (q *qrCode) setFunc(x, y int, dark bool) {
	if x >= 0 && x < q.size && y >= 0 && y < q.size {
		q.modules[y][x] = dark
		q.isFunc[y][x] = true
	}
}

func (q *qrCode) drawFunctionPatterns() {
	q.drawFinder(3, 3)
	q.drawFinder(q.size-4, 3)
	q.drawFinder(3, q.size-4)
	for i := 0; i < q.size; i++ {
		if !q.isFunc[6][i] {
			q.setFunc(i, 6, i%2 == 0)
		}
		if !q.isFunc[i][6] {
			q.setFunc(6, i, i%2 == 0)
		}
	}
	if q.version >= 2 {
		positions := []int{6, q.size - 7}
		for _, y := range positions {
			for _, x := range positions {
				if q.isFunc[y][x] {
					continue
				}
				q.drawAlignment(x, y)
			}
		}
	}
	// Reserve format information areas.
	for i := 0; i < 9; i++ {
		if i != 6 {
			q.setFunc(8, i, false)
			q.setFunc(i, 8, false)
		}
	}
	for i := 0; i < 8; i++ {
		q.setFunc(q.size-1-i, 8, false)
		q.setFunc(8, q.size-1-i, false)
	}
	q.setFunc(8, q.size-8, true) // dark module
}

func (q *qrCode) drawFinder(cx, cy int) {
	for dy := -4; dy <= 4; dy++ {
		for dx := -4; dx <= 4; dx++ {
			x, y := cx+dx, cy+dy
			if x < 0 || x >= q.size || y < 0 || y >= q.size {
				continue
			}
			dist := max(abs(dx), abs(dy))
			q.setFunc(x, y, dist != 2 && dist != 4)
		}
	}
}
func (q *qrCode) drawAlignment(cx, cy int) {
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			d := max(abs(dx), abs(dy))
			q.setFunc(cx+dx, cy+dy, d != 1)
		}
	}
}

func makeDataCodewords(data []byte, dataBytes int) []byte {
	bits := &bitBuffer{}
	bits.append(0x4, 4) // byte mode
	bits.append(uint(len(data)), 8)
	for _, b := range data {
		bits.append(uint(b), 8)
	}
	capBits := dataBytes * 8
	term := min(4, capBits-len(bits.bits))
	bits.append(0, term)
	for len(bits.bits)%8 != 0 {
		bits.append(0, 1)
	}
	out := bits.bytes()
	for pad := byte(0xec); len(out) < dataBytes; {
		out = append(out, pad)
		if pad == 0xec {
			pad = 0x11
		} else {
			pad = 0xec
		}
	}
	return out
}

type bitBuffer struct{ bits []bool }

func (b *bitBuffer) append(val uint, n int) {
	for i := n - 1; i >= 0; i-- {
		b.bits = append(b.bits, ((val>>i)&1) != 0)
	}
}
func (b *bitBuffer) bytes() []byte {
	out := make([]byte, (len(b.bits)+7)/8)
	for i, bit := range b.bits {
		if bit {
			out[i/8] |= 1 << uint(7-i%8)
		}
	}
	return out
}

func (q *qrCode) drawCodewords(codewords []byte) {
	bits := make([]bool, 0, len(codewords)*8)
	for _, b := range codewords {
		for i := 7; i >= 0; i-- {
			bits = append(bits, ((b>>uint(i))&1) != 0)
		}
	}
	i := 0
	for right := q.size - 1; right >= 1; right -= 2 {
		if right == 6 {
			right = 5
		}
		upward := ((right + 1) & 2) == 0
		for vert := 0; vert < q.size; vert++ {
			y := vert
			if upward {
				y = q.size - 1 - vert
			}
			for j := 0; j < 2; j++ {
				x := right - j
				if !q.isFunc[y][x] {
					dark := false
					if i < len(bits) {
						dark = bits[i]
						i++
					}
					q.modules[y][x] = dark
				}
			}
		}
	}
}

func (q *qrCode) applyMask0() {
	for y := 0; y < q.size; y++ {
		for x := 0; x < q.size; x++ {
			if !q.isFunc[y][x] && (x+y)%2 == 0 {
				q.modules[y][x] = !q.modules[y][x]
			}
		}
	}
}

func (q *qrCode) drawFormatBits(mask int) {
	// Error correction level L = 01b.
	data := (1 << 3) | mask
	rem := data << 10
	for i := 14; i >= 10; i-- {
		if ((rem >> uint(i)) & 1) != 0 {
			rem ^= 0x537 << uint(i-10)
		}
	}
	bits := ((data << 10) | (rem & 0x3FF)) ^ 0x5412
	get := func(i int) bool { return ((bits >> uint(i)) & 1) != 0 }
	for i := 0; i <= 5; i++ {
		q.setFunc(8, i, get(i))
	}
	q.setFunc(8, 7, get(6))
	q.setFunc(8, 8, get(7))
	q.setFunc(7, 8, get(8))
	for i := 9; i < 15; i++ {
		q.setFunc(14-i, 8, get(i))
	}
	for i := 0; i < 8; i++ {
		q.setFunc(q.size-1-i, 8, get(i))
	}
	for i := 8; i < 15; i++ {
		q.setFunc(8, q.size-15+i, get(i))
	}
	q.setFunc(8, q.size-8, true)
}

func reedSolomonDivisor(degree int) []byte {
	result := make([]byte, degree)
	result[degree-1] = 1
	root := byte(1)
	for i := 0; i < degree; i++ {
		for j := 0; j < len(result); j++ {
			result[j] = gfMul(result[j], root)
			if j+1 < len(result) {
				result[j] ^= result[j+1]
			}
		}
		root = gfMul(root, 0x02)
	}
	return result
}
func reedSolomonRemainder(data, divisor []byte) []byte {
	result := make([]byte, len(divisor))
	for _, b := range data {
		factor := b ^ result[0]
		copy(result, result[1:])
		result[len(result)-1] = 0
		for i := range result {
			result[i] ^= gfMul(divisor[i], factor)
		}
	}
	return result
}
func gfMul(x, y byte) byte {
	var z int
	a, b := int(x), int(y)
	for b != 0 {
		if b&1 != 0 {
			z ^= a
		}
		a <<= 1
		if a&0x100 != 0 {
			a ^= 0x11d
		}
		b >>= 1
	}
	return byte(z)
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
