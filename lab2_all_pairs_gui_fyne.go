package main

import (
	"encoding/csv"
	"fmt"
	"image/color"
	"math"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

const (
	INF         = 1e18
	MaxVertices = 10
	vertexR     = float32(18)
)

// ---------------- Core graph model ----------------

type Graph struct {
	n       int
	adj     [][]float64
	negEdge bool
}

func NewGraph() *Graph { return &Graph{} }

func (g *Graph) Resize(n int) {
	if n < 0 {
		n = 0
	}
	if n > MaxVertices {
		n = MaxVertices
	}
	old := g.adj
	g.n = n
	g.adj = make([][]float64, n)
	for i := 0; i < n; i++ {
		g.adj[i] = make([]float64, n)
		for j := 0; j < n; j++ {
			if i == j {
				g.adj[i][j] = 0
			} else {
				g.adj[i][j] = INF
			}
		}
	}
	for i := 0; i < n && i < len(old); i++ {
		for j := 0; j < n && j < len(old); j++ {
			g.adj[i][j] = old[i][j]
		}
	}
	g.negEdge = false
	for i := 0; i < g.n; i++ {
		for j := 0; j < g.n; j++ {
			if i != j && g.adj[i][j] < 0 && g.adj[i][j] < INF/2 {
				g.negEdge = true
			}
		}
	}
}

func (g *Graph) SetEdge(i, j int, val float64, isInf bool) {
	if i < 0 || j < 0 || i >= g.n || j >= g.n {
		return
	}
	if i == j {
		g.adj[i][j] = 0
		return
	}
	if isInf {
		g.adj[i][j] = INF
	} else {
		g.adj[i][j] = val
		if val < 0 {
			g.negEdge = true
		}
	}
}

func (g *Graph) HasNegativeEdge() bool { return g.negEdge }

func (g *Graph) FloydWarshall() (dist [][]float64, pred [][]int, negCycle bool) {
	n := g.n
	dist = make([][]float64, n)
	pred = make([][]int, n)
	for i := 0; i < n; i++ {
		dist[i] = make([]float64, n)
		pred[i] = make([]int, n)
		for j := 0; j < n; j++ {
			dist[i][j] = g.adj[i][j]
			if i == j && dist[i][j] == 0 {
				pred[i][j] = i
			} else if dist[i][j] < INF/2 {
				pred[i][j] = i
			} else {
				pred[i][j] = -1
			}
		}
	}
	for k := 0; k < n; k++ {
		for i := 0; i < n; i++ {
			if dist[i][k] >= INF/2 {
				continue
			}
			dik := dist[i][k]
			for j := 0; j < n; j++ {
				if dist[k][j] >= INF/2 {
					continue
				}
				cand := dik + dist[k][j]
				if cand < dist[i][j] {
					dist[i][j] = cand
					pred[i][j] = pred[k][j]
				}
			}
		}
	}
	for i := 0; i < n; i++ {
		if dist[i][i] < 0 {
			return dist, pred, true
		}
	}
	return dist, pred, false
}

func reconstructPathPred(pred [][]int, i, j int) []int {
	if pred[i][j] == -1 {
		return nil
	}
	path := []int{j}
	cur := j
	for cur != i {
		cur = pred[i][cur]
		if cur == -1 {
			return nil
		}
		path = append(path, cur)
	}
	for l, r := 0, len(path)-1; l < r; l, r = l+1, r-1 {
		path[l], path[r] = path[r], path[l]
	}
	for k := range path {
		path[k]++
	}
	return path
}

func (g *Graph) dijkstraFrom(s int) ([]float64, []int) {
	n := g.n
	dist := make([]float64, n)
	prev := make([]int, n)
	used := make([]bool, n)
	for i := 0; i < n; i++ {
		dist[i] = INF
		prev[i] = -1
	}
	dist[s] = 0
	for it := 0; it < n; it++ {
		v := -1
		best := INF
		for i := 0; i < n; i++ {
			if !used[i] && dist[i] < best {
				best = dist[i]
				v = i
			}
		}
		if v == -1 {
			break
		}
		used[v] = true
		for u := 0; u < n; u++ {
			w := g.adj[v][u]
			if w >= INF/2 {
				continue
			}
			if dist[v]+w < dist[u] {
				dist[u] = dist[v] + w
				prev[u] = v
			}
		}
	}
	return dist, prev
}

func reconstructFromPrev(prev []int, s, t int) []int {
	if s == t {
		return []int{s + 1}
	}
	if prev[t] == -1 {
		return nil
	}
	path := []int{}
	cur := t
	for cur != -1 {
		path = append(path, cur)
		if cur == s {
			break
		}
		cur = prev[cur]
	}
	if len(path) == 0 || path[len(path)-1] != s {
		return nil
	}
	for l, r := 0, len(path)-1; l < r; l, r = l+1, r-1 {
		path[l], path[r] = path[r], path[l]
	}
	for i := range path {
		path[i]++
	}
	return path
}

// ---------------- UI helpers ----------------

func infOrFloat(s string) (float64, bool, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return INF, true, nil
	}
	if s == "∞" || strings.EqualFold(s, "inf") {
		return INF, true, nil
	}
	v, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false, fmt.Errorf("некорректное число")
	}
	return v, false, nil
}

func floatToCell(v float64, i, j int) string {
	if i == j {
		return "0"
	}
	if v >= INF/2 {
		return ""
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}

func joinPathInts(p []int) string {
	var b strings.Builder
	for i, v := range p {
		if i > 0 {
			b.WriteString(" → ")
		}
		b.WriteString(strconv.Itoa(v))
	}
	return b.String()
}

// ---------------- Graph editor (clickable canvas) ----------------

type edgeRec struct {
	U, V int
	W    float64
}

type GraphCanvas struct {
	widget.BaseWidget
	verts            []fyne.Position
	edges            []edgeRec
	mode             string // move|addv|adde|delete
	pending          int    // -1 none else start vertex index
	dragIdx          int    // -1 none
	pick             string
	startIdx, endIdx int
	highlightPairs   map[[2]int]bool

	askWeight func(u, v int, done func(w float64, ok bool))
	onChange  func()
}

func NewGraphCanvas() *GraphCanvas {
	gc := &GraphCanvas{mode: "move", pending: -1, dragIdx: -1, startIdx: -1, endIdx: -1}
	gc.ExtendBaseWidget(gc)
	gc.highlightPairs = make(map[[2]int]bool)
	return gc
}

func (gc *GraphCanvas) CreateRenderer() fyne.WidgetRenderer {
	root := container.NewWithoutLayout()
	return &graphRenderer{gc: gc, root: root}
}

type graphRenderer struct {
	gc   *GraphCanvas
	root *fyne.Container
}

func (r *graphRenderer) Layout(s fyne.Size)           { r.root.Resize(s) }
func (r *graphRenderer) MinSize() fyne.Size           { return fyne.NewSize(500, 360) }
func (r *graphRenderer) Destroy()                     {}
func (r *graphRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.root} }
func (r *graphRenderer) Refresh() {
	objs := []fyne.CanvasObject{}
	// edges
	for _, e := range r.gc.edges {
		p1 := r.gc.verts[e.U]
		p2 := r.gc.verts[e.V]
		ln := canvas.NewLine(color.NRGBA{R: 68, G: 68, B: 85, A: 255})
		ln.StrokeWidth = 2
		if r.gc.highlightPairs[[2]int{e.U, e.V}] {
			ln.StrokeColor = color.NRGBA{R: 200, G: 0, B: 0, A: 255}
			ln.StrokeWidth = 3
		}
		ln.Position1 = p1
		ln.Position2 = p2
		mx := (p1.X + p2.X) / 2
		my := (p1.Y + p2.Y) / 2
		txt := canvas.NewText(strconv.FormatFloat(e.W, 'g', -1, 64), color.NRGBA{A: 255})
		txt.TextSize = 12
		txt.Move(fyne.NewPos(mx-8, my-16))
		objs = append(objs, ln, txt)
	}
	// vertices
	for i, p := range r.gc.verts {
		fill := color.NRGBA{R: 232, G: 240, B: 254, A: 255}
		if i == r.gc.startIdx && i == r.gc.endIdx {
			fill = color.NRGBA{R: 253, G: 244, B: 191, A: 255}
		} else if i == r.gc.startIdx {
			fill = color.NRGBA{R: 209, G: 250, B: 223, A: 255}
		} else if i == r.gc.endIdx {
			fill = color.NRGBA{R: 255, G: 232, B: 232, A: 255}
		}
		c := canvas.NewCircle(fill)
		c.StrokeColor = color.NRGBA{R: 86, G: 103, B: 119, A: 255}
		c.StrokeWidth = 2
		c.Resize(fyne.NewSize(vertexR*2, vertexR*2))
		c.Move(fyne.NewPos(p.X-vertexR, p.Y-vertexR))
		label := canvas.NewText(strconv.Itoa(i+1), color.NRGBA{A: 255})
		label.TextStyle = fyne.TextStyle{Bold: true}
		label.TextSize = 12
		label.Move(fyne.NewPos(p.X-4, p.Y-8))
		objs = append(objs, c, label)
	}
	r.root.Objects = objs
	r.root.Refresh()
}

// Helpers for canvas
func (gc *GraphCanvas) findVertex(pos fyne.Position) int {
	for i, p := range gc.verts {
		dx := p.X - pos.X
		dy := p.Y - pos.Y
		if dx*dx+dy*dy <= vertexR*vertexR {
			return i
		}
	}
	return -1
}

func (gc *GraphCanvas) pointSegDist(p fyne.Position, a, b fyne.Position) float32 {
	vx, vy := b.X-a.X, b.Y-a.Y
	wx, wy := p.X-a.X, p.Y-a.Y
	c1 := vx*wx + vy*wy
	if c1 <= 0 {
		dx, dy := p.X-a.X, p.Y-a.Y
		return float32(math.Sqrt(float64(dx*dx + dy*dy)))
	}
	c2 := vx*vx + vy*vy
	if c2 <= c1 {
		dx, dy := p.X-b.X, p.Y-b.Y
		return float32(math.Sqrt(float64(dx*dx + dy*dy)))
	}
	bcoef := c1 / c2
	bx, by := a.X+bcoef*vx, a.Y+bcoef*vy
	dx, dy := p.X-bx, p.Y-by
	return float32(math.Sqrt(float64(dx*dx + dy*dy)))
}

func (gc *GraphCanvas) findEdge(pos fyne.Position) int {
	best, idx := float32(1e9), -1
	for i, e := range gc.edges {
		d := gc.pointSegDist(pos, gc.verts[e.U], gc.verts[e.V])
		if d < best {
			best, idx = d, i
		}
	}
	if best <= 8 {
		return idx
	}
	return -1
}

// Interaction
func (gc *GraphCanvas) Tapped(ev *fyne.PointEvent) {
	if gc.pick == "start" || gc.pick == "end" {
		vid := gc.findVertex(ev.Position)
		if vid != -1 {
			if gc.pick == "start" {
				gc.startIdx = vid
			} else {
				gc.endIdx = vid
			}
			gc.pick = ""
			gc.Refresh()
		}
		return
	}
	switch gc.mode {
	case "addv":
		if len(gc.verts) >= MaxVertices {
			return
		}
		gc.verts = append(gc.verts, ev.Position)
		gc.Refresh()
		if gc.onChange != nil {
			gc.onChange()
		}
	case "adde":
		vid := gc.findVertex(ev.Position)
		if vid == -1 {
			return
		}
		if gc.pending == -1 {
			gc.pending = vid
			return
		}
		if gc.pending == vid {
			gc.pending = -1
			return
		}
		u, v := gc.pending, vid
		gc.pending = -1
		if gc.askWeight != nil {
			gc.askWeight(u, v, func(w float64, ok bool) {
				if !ok {
					return
				}
				for i := range gc.edges {
					if gc.edges[i].U == u && gc.edges[i].V == v {
						gc.edges[i].W = w
						gc.Refresh()
						if gc.onChange != nil {
							gc.onChange()
						}
						return
					}
				}
				gc.edges = append(gc.edges, edgeRec{U: u, V: v, W: w})
				gc.Refresh()
				if gc.onChange != nil {
					gc.onChange()
				}
			})
		}
	case "delete":
		vid := gc.findVertex(ev.Position)
		if vid != -1 {
			gc.deleteVertex(vid)
			gc.Refresh()
			if gc.onChange != nil {
				gc.onChange()
			}
			return
		}
		eidx := gc.findEdge(ev.Position)
		if eidx != -1 {
			gc.edges = append(gc.edges[:eidx], gc.edges[eidx+1:]...)
			gc.Refresh()
			if gc.onChange != nil {
				gc.onChange()
			}
			return
		}
	default:
		// move mode: no tap action
	}
}

func (gc *GraphCanvas) Dragged(ev *fyne.DragEvent) {
	if gc.mode != "move" {
		return
	}
	if gc.dragIdx == -1 {
		vid := gc.findVertex(ev.Position)
		if vid == -1 {
			return
		}
		gc.dragIdx = vid
	}
	gc.verts[gc.dragIdx] = ev.Position
	gc.Refresh()
}

func (gc *GraphCanvas) DragEnd() { gc.dragIdx = -1 }

func (gc *GraphCanvas) deleteVertex(vid int) {
	out := make([]edgeRec, 0, len(gc.edges))
	for _, e := range gc.edges {
		if e.U != vid && e.V != vid {
			out = append(out, e)
		}
	}
	gc.edges = out
	gc.verts = append(gc.verts[:vid], gc.verts[vid+1:]...)
	if gc.startIdx == vid {
		gc.startIdx = -1
	}
	if gc.endIdx == vid {
		gc.endIdx = -1
	}
	for i := range gc.edges {
		if gc.edges[i].U > vid {
			gc.edges[i].U--
		}
		if gc.edges[i].V > vid {
			gc.edges[i].V--
		}
	}
}

func (gc *GraphCanvas) syncToGraph(g *Graph) {
	n := len(gc.verts)
	g.Resize(n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if i != j {
				g.adj[i][j] = INF
			} else {
				g.adj[i][j] = 0
			}
		}
	}
	g.negEdge = false
	for _, e := range gc.edges {
		g.adj[e.U][e.V] = e.W
		if e.W < 0 {
			g.negEdge = true
		}
	}
}

func (gc *GraphCanvas) clearHighlight() {
	gc.highlightPairs = make(map[[2]int]bool)
	gc.Refresh()
}

func (gc *GraphCanvas) setHighlightFromPath1(path1 []int) {
	gc.highlightPairs = make(map[[2]int]bool)
	if len(path1) < 2 {
		gc.Refresh()
		return
	}
	for i := 0; i+1 < len(path1); i++ {
		u := path1[i] - 1
		v := path1[i+1] - 1
		gc.highlightPairs[[2]int{u, v}] = true
	}
	gc.Refresh()
}

// ---------------- Editor window ----------------

func openGraphEditor(a fyne.App, parent fyne.Window, g *Graph, onChanged func()) {
	w := a.NewWindow("Редактор графа (клики)")
	w.Resize(fyne.NewSize(1000, 640))

	gc := NewGraphCanvas()
	gc.onChange = func() {
		gc.syncToGraph(g)
		if onChanged != nil {
			onChanged()
		}
	}
	gc.askWeight = func(u, v int, done func(float64, bool)) {
		entry := widget.NewEntry()
		entry.SetPlaceHolder("вес, например 3.5 или -2")
		d := dialog.NewForm("Вес дуги", "OK", "Отмена", []*widget.FormItem{
			widget.NewFormItem(fmt.Sprintf("%d → %d", u+1, v+1), entry),
		}, func(ok bool) {
			if !ok {
				done(0, false)
				return
			}
			val, err := strconv.ParseFloat(strings.ReplaceAll(entry.Text, ",", "."), 64)
			if err != nil || math.IsNaN(val) || math.IsInf(val, 0) {
				dialog.ShowError(fmt.Errorf("Введите корректное число"), w)
				done(0, false)
				return
			}
			done(val, true)
		}, w)
		d.Show()
	}

	modes := widget.NewRadioGroup([]string{"Перемещать", "Добавлять вершины", "Добавлять дуги", "Удалять"}, func(s string) {
		switch s {
		case "Добавлять вершины":
			gc.mode = "addv"
		case "Добавлять дуги":
			gc.mode = "adde"
		case "Удалять":
			gc.mode = "delete"
		default:
			gc.mode = "move"
		}
	})
	modes.SetSelected("Перемещать")

	btnClear := widget.NewButton("Очистить граф", func() {
		gc.verts = nil
		gc.edges = nil
		gc.pending = -1
		gc.dragIdx = -1
		gc.startIdx = -1
		gc.endIdx = -1
		gc.clearHighlight()
		gc.onChange()
	})

	pickStart := widget.NewButton("Начало", func() {
		gc.pick = "start"
		dialog.ShowInformation("Выбор начальной", "Кликните по вершине на полотне", w)
	})
	pickEnd := widget.NewButton("Конец", func() {
		gc.pick = "end"
		dialog.ShowInformation("Выбор конечной", "Кликните по вершине на полотне", w)
	})
	findPath := widget.NewButton("Найти путь", func() {
		if gc.startIdx == -1 || gc.endIdx == -1 {
			dialog.ShowInformation("Не выбрано", "Сначала выберите начало и конец", w)
			return
		}
		gc.syncToGraph(g)
		if g.HasNegativeEdge() {
			dialog.ShowError(fmt.Errorf("В графе есть отрицательные дуги — алгоритм Дейкстры неприменим"), w)
			return
		}
		d, prev := g.dijkstraFrom(gc.startIdx)
		if d[gc.endIdx] >= INF/2 {
			gc.clearHighlight()
			dialog.ShowInformation("Пути нет", "Между выбранными вершинами пути нет", w)
			return
		}
		path := reconstructFromPrev(prev, gc.startIdx, gc.endIdx)
		gc.setHighlightFromPath1(path)
		msg := fmt.Sprintf("Длина: %g\nПуть: %s", d[gc.endIdx], joinPathInts(path))
		dialog.ShowInformation("Результат", msg, w)
	})
	clearHL := widget.NewButton("Сброс выделения", func() { gc.clearHighlight() })

	left := container.NewVBox(
		modes,
		widget.NewSeparator(),
		btnClear,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Выделение пути (как в ЛР1):", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		pickStart, pickEnd, findPath, clearHL,
	)

	w.SetContent(container.NewBorder(left, nil, nil, nil, container.NewMax(gc)))
	w.Show()
}

// ---------------- Main window (matrix + all-pairs results) ----------------

func main() {
	a := app.New()
	w := a.NewWindow("ЛР №2 — Все пары кратчайших путей (Go/Fyne)")
	w.Resize(fyne.NewSize(1040, 680))

	g := NewGraph()
	g.Resize(4)

	status := binding.NewString()
	status.Set("Готово")

	var buildMatrixGrid func()
	matrixGrid := container.NewVBox()

	nEntry := widget.NewEntry()
	nEntry.SetText("4")
	setNBtn := widget.NewButton("Установить N (≤10)", func() {
		nVal, err := strconv.Atoi(strings.TrimSpace(nEntry.Text))
		if err != nil || nVal < 0 || nVal > MaxVertices {
			dialog.ShowInformation("Ошибка", fmt.Sprintf("N должно быть от 0 до %d", MaxVertices), w)
			return
		}
		g.Resize(nVal)
		status.Set(fmt.Sprintf("Размер матрицы: %d", g.n))
		buildMatrixGrid()
	})

	buildMatrixGrid = func() {
		matrixGrid.Objects = nil
		if g.n == 0 {
			matrixGrid.Add(widget.NewLabel("Матрица пуста — установите N > 0"))
			matrixGrid.Refresh()
			return
		}
		head := container.NewGridWithColumns(g.n + 1)
		head.Add(widget.NewLabel("i/j"))
		for j := 0; j < g.n; j++ {
			head.Add(widget.NewLabel(fmt.Sprintf("%d", j+1)))
		}
		matrixGrid.Add(head)
		for i := 0; i < g.n; i++ {
			row := container.NewGridWithColumns(g.n + 1)
			row.Add(widget.NewLabel(fmt.Sprintf("%d", i+1)))
			for j := 0; j < g.n; j++ {
				cell := widget.NewEntry()
				cell.SetPlaceHolder("∞ = пусто")
				cell.SetText(floatToCell(g.adj[i][j], i, j))
				ci, cj := i, j
				cell.OnChanged = func(s string) {
					v, isInf, err := infOrFloat(s)
					if err != nil {
						return
					}
					g.SetEdge(ci, cj, v, isInf)
				}
				row.Add(cell)
			}
			matrixGrid.Add(row)
		}
		matrixGrid.Refresh()
	}
	buildMatrixGrid()

	// Results table (fast)
	type resRow struct{ I, J, Length, Path string }
	resData := make([]resRow, 0)
	resultsTable := widget.NewTable(
		func() (int, int) { return len(resData), 4 },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.TableCellID, co fyne.CanvasObject) {
			if id.Row >= len(resData) {
				co.(*widget.Label).SetText("")
				return
			}
			r := resData[id.Row]
			var txt string
			switch id.Col {
			case 0:
				txt = r.I
			case 1:
				txt = r.J
			case 2:
				txt = r.Length
			case 3:
				txt = r.Path
			}
			co.(*widget.Label).SetText(txt)
		},
	)
	resultsTable.SetColumnWidth(0, 36)
	resultsTable.SetColumnWidth(1, 36)
	resultsTable.SetColumnWidth(2, 92)
	resultsTable.SetColumnWidth(3, 420)

	updateResults := func(dist [][]float64, getPath func(i, j int) []int) {
		n := len(dist)
		res := make([]resRow, 0, n*(n-1))
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				if i == j {
					continue
				}
				length := "∞"
				if dist[i][j] < INF/2 {
					length = strconv.FormatFloat(dist[i][j], 'g', -1, 64)
				}
				var pathStr string
				if dist[i][j] >= INF/2 {
					pathStr = "пути нет"
				} else {
					p := getPath(i, j)
					if len(p) == 0 {
						pathStr = "-"
					} else {
						var b strings.Builder
						for k, v := range p {
							if k > 0 {
								b.WriteString(" → ")
							}
							b.WriteString(strconv.Itoa(v))
						}
						pathStr = b.String()
					}
				}
				res = append(res, resRow{I: strconv.Itoa(i + 1), J: strconv.Itoa(j + 1), Length: length, Path: pathStr})
			}
		}
		resData = res
		resultsTable.Refresh()
		status.Set(fmt.Sprintf("Готово: %d записей", len(resData)))
	}

	btnFloyd := widget.NewButton("Все пары (Флойд)", func() {
		if g.n == 0 {
			dialog.ShowInformation("Пусто", "Сначала установите N > 0", w)
			return
		}
		dist, pred, neg := g.FloydWarshall()
		if neg {
			dialog.ShowError(fmt.Errorf("Обнаружен отрицательный цикл — решения нет"), w)
			return
		}
		updateResults(dist, func(i, j int) []int { return reconstructPathPred(pred, i, j) })
		status.Set("Флойд: готово")
	})

	btnDij := widget.NewButton("Все пары (n×Дейкстра)", func() {
		if g.n == 0 {
			dialog.ShowInformation("Пусто", "Сначала установите N > 0", w)
			return
		}
		if g.HasNegativeEdge() {
			dialog.ShowError(fmt.Errorf("Есть отрицательные дуги — n×Дейкстра неприменим"), w)
			return
		}
		n := g.n
		dist := make([][]float64, n)
		prevAll := make([][]int, n)
		for s := 0; s < n; s++ {
			d, p := g.dijkstraFrom(s)
			dist[s] = d
			prevAll[s] = p
		}
		updateResults(dist, func(i, j int) []int { return reconstructFromPrev(prevAll[i], i, j) })
		status.Set("n×Дейкстра: готово")
	})

	btnExport := widget.NewButton("Экспорт CSV", func() {
		if g.n == 0 {
			dialog.ShowInformation("Пусто", "Нет данных для экспорта", w)
			return
		}
		dist, pred, neg := g.FloydWarshall()
		if neg {
			dialog.ShowError(fmt.Errorf("Отрицательный цикл — CSV невозможен"), w)
			return
		}
		getPath := func(i, j int) []int { return reconstructPathPred(pred, i, j) }
		dialog.ShowFileSave(func(uc fyne.URIWriteCloser, err error) {
			if err != nil || uc == nil {
				return
			}
			defer uc.Close()
			wrt := csv.NewWriter(uc)
			wrt.Comma = ';'
			wrt.Write([]string{"i", "j", "length", "path"})
			n := len(dist)
			for i := 0; i < n; i++ {
				for j := 0; j < n; j++ {
					if i == j {
						continue
					}
					length := "inf"
					if dist[i][j] < INF/2 {
						length = strconv.FormatFloat(dist[i][j], 'g', -1, 64)
					}
					p := getPath(i, j)
					pathStr := ""
					if len(p) == 0 && dist[i][j] < INF/2 {
						pathStr = "-"
					} else if len(p) > 0 {
						parts := make([]string, len(p))
						for k, v := range p {
							parts[k] = strconv.Itoa(v)
						}
						pathStr = strings.Join(parts, " ")
					}
					wrt.Write([]string{strconv.Itoa(i + 1), strconv.Itoa(j + 1), length, pathStr})
				}
			}
			wrt.Flush()
			if e := wrt.Error(); e != nil {
				dialog.ShowError(e, w)
			} else {
				dialog.ShowInformation("Готово", "CSV сохранён", w)
			}
		}, w)
	})

	btnEditor := widget.NewButton("Редактор графа (клики)…", func() { openGraphEditor(a, w, g, func() { buildMatrixGrid() }) })

	controls := container.NewVBox(
		container.NewHBox(widget.NewLabel("Число вершин N:"), nEntry, setNBtn, btnEditor),
		widget.NewSeparator(),
		container.NewHBox(btnFloyd, btnDij, btnExport),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Матрица весов (∞ — пусто, диагональ 0)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
	)

	split := container.NewVSplit(controls, container.NewVScroll(matrixGrid))
	split.Offset = 0.25

	statusBar := widget.NewLabelWithData(status)
	content := container.NewBorder(nil, statusBar, nil, nil, container.NewHSplit(split, resultsTable))
	w.SetContent(content)
	w.ShowAndRun()
}
