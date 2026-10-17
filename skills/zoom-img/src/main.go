// zoom_img は、画像の一部を切り出し、元画像の px 座標の目盛りを付けて拡大する。細部を確かめるための拡大図を作るもの。
//
//	zoom_img -i 画像 [--at X,Y] [--win px] [--size px] [--mark X,Y ...] [--px X,Y ...] [--smooth] -o 出力
//
// 点 at の周りの win 四方を切り出し、size 四方に拡大する。既定は最近傍で拡大し、画素をそのまま大きく見せる
// （補間で線や縁がにじむと、太さや色を読み違える）。目盛りの数字は元画像の座標で、刻みは窓の大きさで決まる。
// 窓を狭めながら繰り返し、見たい所に寄っていく（coarse-to-fine）。--px の点は RGB を標準出力に出す。
//
// ビルド：cd ~/.claude/skills/zoom-img/src && go build -o ../bin/zoom_img .（スキルに同梱するバイナリ）
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	_ "golang.org/x/image/webp"
)

const usage = `使い方: zoom_img -i 画像 [--at X,Y] [--win px] [--size px] [--mark X,Y ...] [--px X,Y ...] [--smooth] -o 出力

  -i         元の画像
  -o         出力のパス（.png か .jpg）
  --at       切り出す窓の中心（元画像の座標）。省くと画像の中心
  --win      切り出す窓の一辺 px（既定 400）。窓の大きさで目盛りの刻みが変わる（>250 で 50、>60 で 10、それ以下で 5）
  --size     出力の一辺 px（既定 800）
  --mark     赤丸で示す点（元画像の座標）。何度でも渡せる
  --px       RGB を読んで標準出力に出す点（元画像の座標）。何度でも渡せる。出力の図にも小さな印を付ける
  --smooth   補間して拡大する（既定は最近傍。画素をそのまま見たいときは付けない）
`

type pt struct{ x, y int }

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "zoom_img:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	var (
		in, out    string
		at         *pt
		win, size  = 400, 800
		marks, pxs []pt
		smooth     bool
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
		case "--help", "-h":
			fmt.Print(usage)
			return nil
		case "-i", "-o":
			v, err := next()
			if err != nil {
				return err
			}
			if a == "-i" {
				in = v
			} else {
				out = v
			}
		case "--win", "--size":
			v, err := next()
			if err != nil {
				return err
			}
			n, err := strconv.Atoi(v)
			if err != nil || n <= 0 {
				return fmt.Errorf("%s の値が正の数ではありません：%s", a, v)
			}
			if a == "--win" {
				win = n
			} else {
				size = n
			}
		case "--at", "--mark", "--px":
			v, err := next()
			if err != nil {
				return err
			}
			p, err := parsePoint(v)
			if err != nil {
				return fmt.Errorf("%s：%v", a, err)
			}
			switch a {
			case "--at":
				at = &p
			case "--mark":
				marks = append(marks, p)
			default:
				pxs = append(pxs, p)
			}
		case "--smooth":
			smooth = true
		default:
			return fmt.Errorf("知らない引数です：%s\n\n%s", a, usage)
		}
	}
	if in == "" || out == "" {
		return fmt.Errorf("-i と -o が要ります\n\n%s", usage)
	}

	src, err := load(in)
	if err != nil {
		return err
	}
	b := src.Bounds()
	if at == nil {
		at = &pt{b.Min.X + b.Dx()/2, b.Min.Y + b.Dy()/2}
	}
	x0, y0 := at.x-win/2, at.y-win/2
	window := image.Rect(x0, y0, x0+win, y0+win)

	canvas := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.RGBA{128, 128, 128, 255}), image.Point{}, draw.Src) // 画像の外は灰色
	scaler := draw.Interpolator(draw.NearestNeighbor)
	if smooth {
		scaler = draw.CatmullRom
	}
	// 窓のうち画像の内側の部分だけを、出力の対応する位置へ拡大する
	s := float64(size) / float64(win)
	inside := window.Intersect(b)
	if !inside.Empty() {
		dst := image.Rect(int(float64(inside.Min.X-x0)*s), int(float64(inside.Min.Y-y0)*s),
			int(float64(inside.Max.X-x0)*s), int(float64(inside.Max.Y-y0)*s))
		scaler.Scale(canvas, dst, src, inside, draw.Src, nil)
	}

	step := 5
	if win > 250 {
		step = 50
	} else if win > 60 {
		step = 10
	}
	face := numFace(14)
	toOut := func(x, y int) (int, int) { // 元画像の画素 (x, y) の中心が、出力のどこに来るか
		return int((float64(x-x0) + 0.5) * s), int((float64(y-y0) + 0.5) * s)
	}
	drawGrid(canvas, x0, y0, win, step, s, face)
	for _, m := range marks {
		cx, cy := toOut(m.x, m.y)
		ring(canvas, cx, cy, 12, 3, color.RGBA{230, 30, 30, 255})
	}
	for _, p := range pxs {
		cx, cy := toOut(p.x, p.y)
		ring(canvas, cx, cy, 6, 2, color.RGBA{255, 200, 0, 255})
	}

	if err := save(out, canvas); err != nil {
		return err
	}
	fmt.Printf("窓 x=%d..%d y=%d..%d（%dpx 四方、%.2f 倍、目盛り %dpx 刻み、太線 %dpx ごと）\n",
		window.Min.X, window.Max.X, window.Min.Y, window.Max.Y, win, s, step, step*5)
	for _, p := range pxs {
		if !(image.Point{p.x, p.y}).In(b) {
			fmt.Printf("px %d,%d\t画像の外\n", p.x, p.y)
			continue
		}
		r, g, bb, a := src.At(p.x, p.y).RGBA()
		fmt.Printf("px %d,%d\tRGB(%d, %d, %d)\tα=%d\n", p.x, p.y, r>>8, g>>8, bb>>8, a>>8)
	}
	return nil
}

// drawGrid は、元画像の座標で step ごとに細い線、step*5 ごとに太い線を引き、座標の数字を付ける。
// 数字は太い線に付け、線の間隔が出力で 50px 以上あれば細い線にも付ける（窓が狭くても座標が読めるように）。
func drawGrid(dst *image.RGBA, x0, y0, win, step int, s float64, face font.Face) {
	minor := color.NRGBA{120, 180, 255, 150}
	major := color.NRGBA{0, 110, 255, 220}
	size := dst.Bounds().Dx()
	labelAll := float64(step)*s >= 50
	for g := ceilTo(x0, step); g < x0+win; g += step {
		X := int(float64(g-x0) * s)
		isMajor := g%(step*5) == 0
		vline(dst, X, size, pick(isMajor, major, minor), pick(isMajor, 2, 1))
		if isMajor || labelAll {
			label(dst, X+3, 3, strconv.Itoa(g), face)
		}
	}
	for g := ceilTo(y0, step); g < y0+win; g += step {
		Y := int(float64(g-y0) * s)
		isMajor := g%(step*5) == 0
		hline(dst, Y, size, pick(isMajor, major, minor), pick(isMajor, 2, 1))
		if isMajor || labelAll {
			label(dst, 3, Y+3, strconv.Itoa(g), face)
		}
	}
}

func ceilTo(v, step int) int {
	q := v / step * step
	if q < v {
		q += step
	}
	return q
}

func pick[T any](c bool, a, b T) T {
	if c {
		return a
	}
	return b
}

func vline(dst *image.RGBA, x, h int, c color.Color, w int) {
	draw.Draw(dst, image.Rect(x, 0, x+w, h), image.NewUniform(c), image.Point{}, draw.Over)
}

func hline(dst *image.RGBA, y, w int, c color.Color, t int) {
	draw.Draw(dst, image.Rect(0, y, w, y+t), image.NewUniform(c), image.Point{}, draw.Over)
}

// label は、白地に濃い青の数字を載せる（どんな絵の上でも読めるように）。
func label(dst *image.RGBA, x, y int, text string, face font.Face) {
	m := face.Metrics()
	tw := font.MeasureString(face, text).Ceil()
	box := image.Rect(x, y, x+tw+4, y+m.Height.Ceil()+2)
	draw.Draw(dst, box, image.NewUniform(color.NRGBA{255, 255, 255, 210}), image.Point{}, draw.Over)
	d := &font.Drawer{Dst: dst, Src: image.NewUniform(color.RGBA{0, 70, 190, 255}), Face: face,
		Dot: fixed.P(x+2, y+1+m.Ascent.Ceil())}
	d.DrawString(text)
}

// ring は、中心 (cx, cy)・半径 r・太さ t の輪を描く。
func ring(dst *image.RGBA, cx, cy, r, t int, c color.Color) {
	for y := cy - r - t; y <= cy+r+t; y++ {
		for x := cx - r - t; x <= cx+r+t; x++ {
			dx, dy := x-cx, y-cy
			d2 := dx*dx + dy*dy
			if d2 >= r*r && d2 <= (r+t)*(r+t) && (image.Point{x, y}).In(dst.Bounds()) {
				dst.Set(x, y, c)
			}
		}
	}
}

func numFace(size float64) font.Face {
	f, _ := opentype.Parse(gobold.TTF)
	face, _ := opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	return face
}

func parsePoint(s string) (pt, error) {
	parts := strings.Split(s, ",")
	if len(parts) != 2 {
		return pt{}, fmt.Errorf("X,Y の形で渡してください：%s", s)
	}
	x, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	y, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil {
		return pt{}, fmt.Errorf("数が読めません：%s", s)
	}
	return pt{x, y}, nil
}

// load は画像を読む。拡張子が無いパスは .png を補う（compare_imgs と同じ）。
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
		err = jpeg.Encode(f, img, &jpeg.Options{Quality: 90})
	default:
		err = png.Encode(f, img)
	}
	if err != nil {
		return err
	}
	fmt.Printf("%s  %dx%d\n", path, img.Bounds().Dx(), img.Bounds().Dy())
	return nil
}
