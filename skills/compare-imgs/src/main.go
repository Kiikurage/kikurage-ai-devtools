// compare_imgs は、任意の枚数の画像をラベル付きで 1 枚に並べる。見比べるための一覧を作るもの。
//
//	compare_imgs [-w 幅px] [-h 高さpx] -o 出力 -i 画像1 [ラベル1] -i 画像2 [ラベル2] ...
//
// 全体の大きさ（既定 2400x1600）の縦横比をもとに、何行に分けると画像がいちばん大きく収まるかを選び、
// 行ごとに高さを揃えて横幅いっぱいに並べてから、全体に収まるように縮める。出力は中身の大きさに切り詰める。
// 各画像の左上に通し番号とラベル、左下にパスを載せ（日本語はヒラギノを使う）、標準出力にも番号・位置・パス・ラベルを出す。
//
// ビルド：cd ~/.claude/skills/compare-imgs/src && go build -o ../bin/compare_imgs .（スキルに同梱するバイナリ）
package main

import (
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	_ "golang.org/x/image/webp"
	"golang.org/x/text/unicode/norm"
)

const usage = `使い方: compare_imgs [-w 幅px] [-h 高さpx] [--gap px] [--bg 色] [--font パス] -o 出力 -i 画像 [ラベル] ...

  -w, -h     全体の最大の大きさ（既定 2400x1600）。片方だけなら、もう片方は 2:3 の比で決める
  -o         出力のパス（.png か .jpg）
  -i         画像のパスと、任意のラベル。ラベルは次の引数が - で始まらなければラベルとみなす
  --gap      画像の間の余白（既定 8）
  --bg       背景色（white / black / #rrggbb。既定 white）
  --font     ラベルのフォント（.ttf / .otf / .ttc。既定はヒラギノ角ゴシック W6、無ければ同梱の BIZ UDPゴシック）
`

type item struct {
	path  string
	label string
	img   image.Image
	ar    float64 // 幅 / 高さ
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "compare_imgs:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	var (
		w, h     int
		gap      = 8
		out      string
		bg       color.Color = color.White
		fontPath string
		items    []*item
	)
	for i := 0; i < len(args); i++ {
		a := args[i]
		next := func() (string, error) {
			if i+1 >= len(args) {
				return "", fmt.Errorf("%s に値がありません", a)
			}
			i++
			return args[i], nil
		}
		switch a {
		case "--help":
			fmt.Print(usage)
			return nil
		case "-w", "-h", "--gap":
			v, err := next()
			if err != nil {
				return err
			}
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				return fmt.Errorf("%s の値が数ではありません：%s", a, v)
			}
			switch a {
			case "-w":
				w = n
			case "-h":
				h = n
			default:
				gap = n
			}
		case "-o":
			v, err := next()
			if err != nil {
				return err
			}
			out = v
		case "--bg":
			v, err := next()
			if err != nil {
				return err
			}
			c, err := parseColor(v)
			if err != nil {
				return err
			}
			bg = c
		case "--font":
			v, err := next()
			if err != nil {
				return err
			}
			fontPath = v
		case "-i":
			v, err := next()
			if err != nil {
				return err
			}
			it := &item{path: v}
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				it.label = args[i]
			}
			items = append(items, it)
		default:
			return fmt.Errorf("知らない引数です：%s\n\n%s", a, usage)
		}
	}
	if out == "" || len(items) == 0 {
		return fmt.Errorf("-o と、1 つ以上の -i が要ります\n\n%s", usage)
	}
	switch {
	case w == 0 && h == 0:
		w, h = 2400, 1600
	case h == 0:
		h = w * 2 / 3
	case w == 0:
		w = h * 3 / 2
	}

	for _, it := range items {
		img, err := load(it.path)
		if err != nil {
			return err
		}
		it.img = img
		b := img.Bounds()
		it.ar = float64(b.Dx()) / float64(b.Dy())
	}

	rows, scale := bestLayout(items, w, h, gap)
	canvas, cells := render(rows, scale, w, gap, bg, labelFace(fontPath, 26), labelFace(fontPath, 16))
	if err := save(out, canvas); err != nil {
		return err
	}
	// どこにどの画像を置いたかを出す（画像の中の文字は、読む側によっては拾いにくいので）
	for _, c := range cells {
		r := c.rect
		line := fmt.Sprintf("%d\t%d行目 %d列目\tx=%d y=%d %dx%d\t%s", c.no, c.row, c.col, r.Min.X, r.Min.Y, r.Dx(), r.Dy(), c.it.path)
		if c.it.label != "" {
			line += "\t" + c.it.label
		}
		fmt.Println(line)
	}
	return nil
}

// load は画像を読む。拡張子が無いパスは .png を補う（playground の runId/sSEED のような書き方のため）。
func load(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil && filepath.Ext(path) == "" {
		f, err = os.Open(path + ".png")
	}
	if err != nil {
		return nil, fmt.Errorf("開けません：%s", path)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("画像として読めません：%s（%v）", path, err)
	}
	return img, nil
}

// bestLayout は、行の分け方の候補ごとに、全体に収まる倍率と、各画像の面積の対数の和を求め、和がいちばん大きいものを選ぶ。
// 面積の合計で選ぶと 1 枚だけ大きく残りが小さい並びになるので、どの画像もなるべく大きく揃うようにする。
// 返すのは、行ごとの画像と、幅 w で並べたときの寸法を全体に収めるための倍率。
func bestLayout(items []*item, w, h, gap int) ([][]*item, float64) {
	var best [][]*item
	bestScale, bestScore := 0.0, math.Inf(-1)
	for _, rows := range candidates(items) {
		// 余白は縮めないので、画像の部分の高さだけで倍率を決める（幅は 1 以下の倍率なら必ず収まる）
		sum := 0.0
		for _, row := range rows {
			sum += rowHeight(row, w, gap)
		}
		scale := math.Min(1, float64(h-gap*(len(rows)-1))/sum)
		if scale <= 0 {
			continue
		}
		score := 0.0
		for _, row := range rows {
			rh := rowHeight(row, w, gap) * scale
			for _, it := range row {
				score += math.Log(rh * rh * it.ar)
			}
		}
		if score > bestScore {
			best, bestScale, bestScore = rows, scale, score
		}
	}
	return best, bestScale
}

// candidates は、並びの順を保った行の分け方の候補。16 枚までは全通り、それより多ければ行数ごとに 1 通り（partition）。
func candidates(items []*item) [][][]*item {
	n := len(items)
	var out [][][]*item
	if n <= 16 {
		for mask := 0; mask < 1<<(n-1); mask++ {
			var rows [][]*item
			start := 0
			for i := 1; i < n; i++ {
				if mask&(1<<(i-1)) != 0 {
					rows = append(rows, items[start:i])
					start = i
				}
			}
			out = append(out, append(rows, items[start:]))
		}
		return out
	}
	for r := 1; r <= n; r++ {
		out = append(out, partition(items, r))
	}
	return out
}

// partition は、並びの順を保ったまま r 行に分ける。各行の縦横比の和がなるべく揃うように（線形分割の DP）。
func partition(items []*item, r int) [][]*item {
	n := len(items)
	pre := make([]float64, n+1)
	for i, it := range items {
		pre[i+1] = pre[i] + it.ar
	}
	// cost[k][i]：先頭 i 枚を k 行に分けたときの、行の縦横比の和の最大値（小さいほど揃っている）
	inf := math.Inf(1)
	cost := make([][]float64, r+1)
	cut := make([][]int, r+1)
	for k := range cost {
		cost[k] = make([]float64, n+1)
		cut[k] = make([]int, n+1)
		for i := range cost[k] {
			cost[k][i] = inf
		}
	}
	cost[0][0] = 0
	for k := 1; k <= r; k++ {
		for i := k; i <= n; i++ {
			for j := k - 1; j < i; j++ {
				c := math.Max(cost[k-1][j], pre[i]-pre[j])
				if c < cost[k][i] {
					cost[k][i], cut[k][i] = c, j
				}
			}
		}
	}
	rows := make([][]*item, r)
	for k, i := r, n; k > 0; k-- {
		j := cut[k][i]
		rows[k-1] = items[j:i]
		i = j
	}
	return rows
}

// rowHeight は、行の画像を高さを揃えて幅 w いっぱいに並べたときの高さ。
func rowHeight(row []*item, w, gap int) float64 {
	sum := 0.0
	for _, it := range row {
		sum += it.ar
	}
	return (float64(w) - float64(gap*(len(row)-1))) / sum
}

type cell struct {
	it           *item
	no, row, col int // 通し番号と位置（どれも 1 から）
	rect         image.Rectangle
}

// render は、行ごとに画像を並べて描く。余白は縮めず、画像の部分だけを scale で縮める。
// 寸法は切り捨てて、全体の幅 w を超えないようにする。各画像の左上に通し番号とラベル、左下にパスを載せる。
func render(rows [][]*item, scale float64, w, gap int, bg color.Color, face, small font.Face) (*image.RGBA, []cell) {
	var cells []cell
	y, maxW, no := 0, 0, 0
	for ri, row := range rows {
		rh := int(rowHeight(row, w, gap) * scale)
		x := 0
		for ci, it := range row {
			no++
			cw := int(float64(rh) * it.ar)
			cells = append(cells, cell{it, no, ri + 1, ci + 1, image.Rect(x, y, x+cw, y+rh)})
			x += cw + gap
		}
		if x-gap > maxW {
			maxW = x - gap
		}
		y += rh + gap
	}
	canvas := image.NewRGBA(image.Rect(0, 0, maxW, y-gap))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(bg), image.Point{}, draw.Src)
	for _, c := range cells {
		draw.CatmullRom.Scale(canvas, c.rect, c.it.img, c.it.img.Bounds(), draw.Over, nil)
		label := strconv.Itoa(c.no)
		if c.it.label != "" {
			label += "  " + c.it.label
		}
		drawLabel(canvas, c.rect, label, face, false, false)
		drawLabel(canvas, c.rect, c.it.path, small, true, true)
	}
	return canvas, cells
}

// drawLabel は、画像の左上（bottom なら左下）に半透明の黒い帯を敷いて白い文字を載せる。
// 文字が画像より長ければ、末尾（keepTail なら頭）を … で切る。
func drawLabel(dst *image.RGBA, cell image.Rectangle, label string, face font.Face, bottom, keepTail bool) {
	// 帯の高さは、行の高さ（行間込み）ではなく、実際の字の上端から下端までで決める。
	// どのラベルでも同じ高さになるよう、上に出る字と下に出る字を含む見本の文字列で測る
	b, _ := font.BoundString(face, "Ag漢jあ")
	above, below := -b.Min.Y.Floor(), b.Max.Y.Ceil()
	pad := (above + below) / 4
	maxW := fixed.I(cell.Dx() - pad*2)
	text := label
	for font.MeasureString(face, text) > maxW && len([]rune(text)) > 1 {
		r := []rune(text)
		if keepTail {
			text = "…" + string(r[2:])
		} else {
			text = string(r[:len(r)-2]) + "…"
		}
	}
	tw := font.MeasureString(face, text).Ceil()
	bh := above + below + pad*2
	top := cell.Min.Y
	if bottom {
		top = cell.Max.Y - bh
	}
	box := image.Rect(cell.Min.X, top, cell.Min.X+tw+pad*2, top+bh).Intersect(cell)
	draw.Draw(dst, box, image.NewUniform(color.NRGBA{0, 0, 0, 160}), image.Point{}, draw.Over)
	d := &font.Drawer{Dst: dst, Src: image.White, Face: face,
		Dot: fixed.P(cell.Min.X+pad, top+pad+above)}
	d.DrawString(text)
}

// labelFace はラベルのフォントを読む。指定が無ければヒラギノ、それも無ければ Go の欧文フォント（日本語は出ない）。
// BIZ UDPゴシック Bold（SIL OFL 1.1。ライセンスは fonts/OFL.txt）
//
//go:embed fonts/BIZUDPGothic-Bold.ttf
var embeddedFont []byte

func labelFace(path string, size float64) font.Face {
	opts := &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull}
	candidates := []string{path}
	if path == "" {
		candidates = []string{
			"/System/Library/Fonts/" + norm.NFD.String("ヒラギノ角ゴシック W6.ttc"),
			"/System/Library/Fonts/" + norm.NFC.String("ヒラギノ角ゴシック W6.ttc"),
			"/System/Library/Fonts/Hiragino Sans GB.ttc",
		}
	}
	for _, p := range candidates {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var f *opentype.Font
		if c, err := opentype.ParseCollection(data); err == nil {
			f, err = c.Font(0)
			if err != nil {
				continue
			}
		} else if f, err = opentype.Parse(data); err != nil {
			continue
		}
		if face, err := opentype.NewFace(f, opts); err == nil {
			return face
		}
	}
	// ヒラギノが無い環境（Linux など）では、埋め込んだ BIZ UDPゴシック を使う
	f, _ := opentype.Parse(embeddedFont)
	face, _ := opentype.NewFace(f, opts)
	return face
}

func parseColor(s string) (color.Color, error) {
	switch strings.ToLower(s) {
	case "white":
		return color.White, nil
	case "black":
		return color.Black, nil
	}
	var r, g, b uint8
	if _, err := fmt.Sscanf(strings.TrimPrefix(s, "#"), "%02x%02x%02x", &r, &g, &b); err != nil {
		return nil, fmt.Errorf("色が読めません：%s", s)
	}
	return color.RGBA{r, g, b, 255}, nil
}

func save(path string, img image.Image) error {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		err = jpeg.Encode(f, img, &jpeg.Options{Quality: 88})
	default:
		err = png.Encode(f, img)
	}
	if err != nil {
		return err
	}
	fmt.Printf("%s  %dx%d\n", path, img.Bounds().Dx(), img.Bounds().Dy())
	return nil
}
