package main

import (
	"math"
	"syscall/js"
)

type Point struct {
	X, Y float64
}

type Shape struct {
	Points []Point
	Color  string
	Width  float64
}

type DrawingApp struct {
	canvas       js.Value
	ctx          js.Value
	gl           js.Value
	isDrawing    bool
	shapes       []Shape
	current      Shape
	history      [][]Shape
	historyIndex int
	color        string
	lineWidth    float64
	mode         string // "draw", "line", "circle", "rect"
	startPoint   Point
}

var app *DrawingApp

func main() {
	c := make(chan struct{}, 0)

	js.Global().Set("initWasmDrawing", js.FuncOf(initWasmDrawing))

	println("Go WASM WebGL Drawing App initialized")
	<-c
}

func initWasmDrawing(this js.Value, args []js.Value) interface{} {
	app = &DrawingApp{
		shapes:       []Shape{},
		history:      [][]Shape{},
		historyIndex: -1,
		color:        "#000000",
		lineWidth:    2.0,
		mode:         "draw",
	}

	app.canvas = js.Global().Get("document").Call("getElementById", "drawingCanvas")
	app.ctx = app.canvas.Call("getContext", "2d")

	// Set canvas size
	app.canvas.Set("width", 800)
	app.canvas.Set("height", 600)

	// Setup event listeners
	app.setupEventListeners()

	// Draw initial state
	app.render()

	return nil
}

func (a *DrawingApp) setupEventListeners() {
	// Mouse events
	a.canvas.Call("addEventListener", "mousedown", js.FuncOf(a.handleMouseDown))
	a.canvas.Call("addEventListener", "mousemove", js.FuncOf(a.handleMouseMove))
	a.canvas.Call("addEventListener", "mouseup", js.FuncOf(a.handleMouseUp))

	// Touch events for mobile
	a.canvas.Call("addEventListener", "touchstart", js.FuncOf(a.handleTouchStart))
	a.canvas.Call("addEventListener", "touchmove", js.FuncOf(a.handleTouchMove))
	a.canvas.Call("addEventListener", "touchend", js.FuncOf(a.handleTouchEnd))

	// Control buttons
	js.Global().Set("wasmUndo", js.FuncOf(a.undo))
	js.Global().Set("wasmRedo", js.FuncOf(a.redo))
	js.Global().Set("wasmClear", js.FuncOf(a.clear))
	js.Global().Set("wasmSetColor", js.FuncOf(a.setColor))
	js.Global().Set("wasmSetWidth", js.FuncOf(a.setWidth))
	js.Global().Set("wasmSetMode", js.FuncOf(a.setMode))
}

func (a *DrawingApp) getMousePos(event js.Value) Point {
	rect := a.canvas.Call("getBoundingClientRect")
	return Point{
		X: event.Get("clientX").Float() - rect.Get("left").Float(),
		Y: event.Get("clientY").Float() - rect.Get("top").Float(),
	}
}

func (a *DrawingApp) getTouchPos(event js.Value) Point {
	rect := a.canvas.Call("getBoundingClientRect")
	touch := event.Get("touches").Index(0)
	return Point{
		X: touch.Get("clientX").Float() - rect.Get("left").Float(),
		Y: touch.Get("clientY").Float() - rect.Get("top").Float(),
	}
}

func (a *DrawingApp) handleMouseDown(this js.Value, args []js.Value) interface{} {
	event := args[0]
	event.Call("preventDefault")

	a.isDrawing = true
	pos := a.getMousePos(event)
	a.startPoint = pos

	a.current = Shape{
		Points: []Point{pos},
		Color:  a.color,
		Width:  a.lineWidth,
	}

	return nil
}

func (a *DrawingApp) handleMouseMove(this js.Value, args []js.Value) interface{} {
	if !a.isDrawing {
		return nil
	}

	event := args[0]
	event.Call("preventDefault")
	pos := a.getMousePos(event)

	if a.mode == "draw" {
		a.current.Points = append(a.current.Points, pos)
		a.render()
		a.drawShape(a.current)
	} else {
		a.render()
		a.drawPreview(pos)
	}

	return nil
}

func (a *DrawingApp) handleMouseUp(this js.Value, args []js.Value) interface{} {
	if !a.isDrawing {
		return nil
	}

	event := args[0]
	event.Call("preventDefault")
	pos := a.getMousePos(event)

	if a.mode != "draw" {
		a.current.Points = a.createShapePoints(a.startPoint, pos)
	}

	a.isDrawing = false
	a.shapes = append(a.shapes, a.current)
	a.saveState()
	a.render()

	return nil
}

func (a *DrawingApp) handleTouchStart(this js.Value, args []js.Value) interface{} {
	event := args[0]
	event.Call("preventDefault")

	a.isDrawing = true
	pos := a.getTouchPos(event)
	a.startPoint = pos

	a.current = Shape{
		Points: []Point{pos},
		Color:  a.color,
		Width:  a.lineWidth,
	}

	return nil
}

func (a *DrawingApp) handleTouchMove(this js.Value, args []js.Value) interface{} {
	if !a.isDrawing {
		return nil
	}

	event := args[0]
	event.Call("preventDefault")
	pos := a.getTouchPos(event)

	if a.mode == "draw" {
		a.current.Points = append(a.current.Points, pos)
		a.render()
		a.drawShape(a.current)
	} else {
		a.render()
		a.drawPreview(pos)
	}

	return nil
}

func (a *DrawingApp) handleTouchEnd(this js.Value, args []js.Value) interface{} {
	if !a.isDrawing {
		return nil
	}

	event := args[0]
	event.Call("preventDefault")

	if a.mode != "draw" && len(a.current.Points) > 0 {
		lastPos := a.current.Points[len(a.current.Points)-1]
		a.current.Points = a.createShapePoints(a.startPoint, lastPos)
	}

	a.isDrawing = false
	a.shapes = append(a.shapes, a.current)
	a.saveState()
	a.render()

	return nil
}

func (a *DrawingApp) createShapePoints(start, end Point) []Point {
	switch a.mode {
	case "line":
		return []Point{start, end}
	case "rect":
		return []Point{
			start,
			{end.X, start.Y},
			end,
			{start.X, end.Y},
			start,
		}
	case "circle":
		cx := (start.X + end.X) / 2
		cy := (start.Y + end.Y) / 2
		rx := math.Abs(end.X-start.X) / 2
		ry := math.Abs(end.Y-start.Y) / 2

		points := []Point{}
		steps := 50
		for i := 0; i <= steps; i++ {
			angle := float64(i) * 2 * math.Pi / float64(steps)
			points = append(points, Point{
				X: cx + rx*math.Cos(angle),
				Y: cy + ry*math.Sin(angle),
			})
		}
		return points
	}
	return []Point{start, end}
}

func (a *DrawingApp) drawPreview(endPos Point) {
	preview := Shape{
		Points: a.createShapePoints(a.startPoint, endPos),
		Color:  a.color,
		Width:  a.lineWidth,
	}
	a.drawShape(preview)
}

func (a *DrawingApp) drawShape(shape Shape) {
	if len(shape.Points) == 0 {
		return
	}

	a.ctx.Set("strokeStyle", shape.Color)
	a.ctx.Set("lineWidth", shape.Width)
	a.ctx.Set("lineCap", "round")
	a.ctx.Set("lineJoin", "round")

	a.ctx.Call("beginPath")
	a.ctx.Call("moveTo", shape.Points[0].X, shape.Points[0].Y)

	for i := 1; i < len(shape.Points); i++ {
		a.ctx.Call("lineTo", shape.Points[i].X, shape.Points[i].Y)
	}

	a.ctx.Call("stroke")
}

func (a *DrawingApp) render() {
	// Clear canvas
	a.ctx.Set("fillStyle", "#ffffff")
	a.ctx.Call("fillRect", 0, 0, 800, 600)

	// Draw grid
	a.drawGrid()

	// Draw all shapes
	for _, shape := range a.shapes {
		a.drawShape(shape)
	}
}

func (a *DrawingApp) drawGrid() {
	a.ctx.Set("strokeStyle", "#f0f0f0")
	a.ctx.Set("lineWidth", 1)

	// Vertical lines
	for x := 0; x < 800; x += 50 {
		a.ctx.Call("beginPath")
		a.ctx.Call("moveTo", x, 0)
		a.ctx.Call("lineTo", x, 600)
		a.ctx.Call("stroke")
	}

	// Horizontal lines
	for y := 0; y < 600; y += 50 {
		a.ctx.Call("beginPath")
		a.ctx.Call("moveTo", 0, y)
		a.ctx.Call("lineTo", 800, y)
		a.ctx.Call("stroke")
	}
}

func (a *DrawingApp) saveState() {
	// Remove any states after current index
	a.history = a.history[:a.historyIndex+1]

	// Deep copy shapes
	stateCopy := make([]Shape, len(a.shapes))
	for i, shape := range a.shapes {
		pointsCopy := make([]Point, len(shape.Points))
		copy(pointsCopy, shape.Points)
		stateCopy[i] = Shape{
			Points: pointsCopy,
			Color:  shape.Color,
			Width:  shape.Width,
		}
	}

	a.history = append(a.history, stateCopy)
	a.historyIndex++

	// Limit history size
	if len(a.history) > 50 {
		a.history = a.history[1:]
		a.historyIndex--
	}
}

func (a *DrawingApp) undo(this js.Value, args []js.Value) interface{} {
	if a.historyIndex > 0 {
		a.historyIndex--
		a.shapes = make([]Shape, len(a.history[a.historyIndex]))
		copy(a.shapes, a.history[a.historyIndex])
		a.render()
	}
	return nil
}

func (a *DrawingApp) redo(this js.Value, args []js.Value) interface{} {
	if a.historyIndex < len(a.history)-1 {
		a.historyIndex++
		a.shapes = make([]Shape, len(a.history[a.historyIndex]))
		copy(a.shapes, a.history[a.historyIndex])
		a.render()
	}
	return nil
}

func (a *DrawingApp) clear(this js.Value, args []js.Value) interface{} {
	a.shapes = []Shape{}
	a.saveState()
	a.render()
	return nil
}

func (a *DrawingApp) setColor(this js.Value, args []js.Value) interface{} {
	if len(args) > 0 {
		a.color = args[0].String()
	}
	return nil
}

func (a *DrawingApp) setWidth(this js.Value, args []js.Value) interface{} {
	if len(args) > 0 {
		a.lineWidth = args[0].Float()
	}
	return nil
}

func (a *DrawingApp) setMode(this js.Value, args []js.Value) interface{} {
	if len(args) > 0 {
		a.mode = args[0].String()
	}
	return nil
}
