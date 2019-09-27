package main

import (
	"flag"
	"fmt"
	"image/color"
	"log"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/faiface/pixel"
	"github.com/faiface/pixel/pixelgl"
	"golang.org/x/image/colornames"
)

func main() {
	pixelgl.Run(run)
}

type cellDiff struct {
	row int
	col int
}

func run() {

	var rows int
	var cols int

	// the dimensions of a logical cell in terms of pixels on the screen
	var cellWidthPixels int
	var cellHeightPixels int

	// chance of a cell being alive from the start is 1 / aliveRate
	var aliveRate int

	// rate at which a new state is calculated in order to be displayed, we don't want to display very rapid state changes
	var tickRate time.Duration

	rows, cols, cellWidthPixels, cellHeightPixels, aliveRate, tickRate = initVars()

	fmt.Printf("rows: %d cols: %d cellWidthPixels: %d cellHeightPixels: %d aliveRate: %d tickRate: %s\n", rows, cols, cellWidthPixels, cellHeightPixels, aliveRate, tickRate)

	grid1 := make([][]bool, rows)
	grid2 := make([][]bool, rows)

	for i := 0; i < rows; i++ {
		grid1[i] = make([]bool, cols)
		grid2[i] = make([]bool, cols)
	}

	gridDrawer := gridDrawer{
		rows:             rows,
		cols:             cols,
		cellWidthPixels:  cellWidthPixels,
		cellHeightPixels: cellHeightPixels,
	}
	gridDrawer.init()

	// changeLists[i] is the changes that were applied to state i-1 to get to state i
	// i.e.: changeLists[0] is the seed changes that were applied to an completely dead grid
	changeLists := make([][]cellDiff, 0, 512)

	rand.Seed(time.Now().UnixNano())

	initialChanges := seedGrid(grid1, aliveRate)

	lastChangesIdx := -1

	var paused bool

	window, err := pixelgl.NewWindow(pixelgl.WindowConfig{
		Title: "Life",
		Bounds: pixel.Rect{
			Min: pixel.Vec{
				X: 0,
				Y: 0,
			},
			Max: pixel.Vec{
				X: float64(cols * cellWidthPixels),
				Y: float64(rows * cellHeightPixels),
			},
		},
		//VSync: true,
		Resizable: true,
	})

	if err != nil {
		log.Fatal(err)
	}

	ticker := time.NewTicker(tickRate)

	fpsTicker := time.Tick(1 * time.Second)
	var fps uint64

	// do FPS drawing to title bar in a separate go routine, this routine sleeps 99% of the time
	go func() {
		for {
			<-fpsTicker
			window.SetTitle(fmt.Sprintf("FPS: %d", atomic.LoadUint64(&fps)))
			atomic.StoreUint64(&fps, 0)
		}
	}()

	window.Clear(colornames.White)

	gridDrawer.drawDiff(grid1, initialChanges, window)

	window.Update()

	for !window.Closed() {

		// handle possible resizing of window
		changed := gridDrawer.maybeReconfigure(window)
		if changed {
			gridDrawer.drawGrid(grid1, window)
		}

		if window.JustPressed(pixelgl.KeySpace) {
			paused = !paused

		} else if window.JustPressed(pixelgl.KeyRight) && paused {
			// can't reuse a saved diff, make a move and then use those changes
			if lastChangesIdx == len(changeLists)-1 {
				changeList := doTurn(grid1, grid2)

				grid1, grid2 = grid2, grid1

				changeLists = append(changeLists, changeList)
				lastChangesIdx++

				gridDrawer.drawDiff(grid1, changeList, window)

			} else { // can reuse a saved diff here

				lastChangesIdx++

				changeList := changeLists[lastChangesIdx]

				applyChanges(grid1, changeList)

				gridDrawer.drawDiff(grid1, changeList, window)
			}

		} else if window.JustPressed(pixelgl.KeyLeft) && paused {

			// apply the diffs at lastChangeIdx to the grid & then draw using that grid & diffs
			if lastChangesIdx >= 0 {
				changeList := changeLists[lastChangesIdx]

				applyChanges(grid1, changeList)
				gridDrawer.drawDiff(grid1, changeList, window)
				lastChangesIdx--
			}

		} else if window.JustPressed(pixelgl.KeyLeftShift) {

			changeLists = changeLists[:0]

			newInitialChanges := seedGrid(grid1, aliveRate)

			changeLists = append(changeLists, newInitialChanges)

			window.Clear(colornames.White)

			gridDrawer.drawDiff(grid1, newInitialChanges, window)

		} else if window.JustPressed(pixelgl.KeyComma) {
			ticker.Stop()
			tickRate = minDuration(50*time.Millisecond, tickRate/2)
			ticker = time.NewTicker(tickRate)

		} else if window.JustPressed(pixelgl.KeyPeriod) {
			ticker.Stop()
			tickRate *= 2
			ticker = time.NewTicker(tickRate)
		}

		select {
		case <-ticker.C:
			if !paused {
				changeList := doTurn(grid1, grid2)

				changeLists = append(changeLists, changeList)
				lastChangesIdx++

				// swap the buffers so that grid1 has the new state
				grid1, grid2 = grid2, grid1

				gridDrawer.drawDiff(grid1, changeList, window)
			}
		default:
		}

		window.Update()

		atomic.AddUint64(&fps, 1)
	}

	if doTurnCalls == 0 {
		fmt.Println("no doTurn calls were made")
	} else {
		fmt.Printf("average do turn time: %s\n", totalDoTurnTime/time.Duration(doTurnCalls))
	}

	if drawCalls == 0 {
		fmt.Println("no draw calls were made")
	} else {
		fmt.Printf("average draw time: %s\n", totalDrawTime/time.Duration(drawCalls))
	}
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// seedGrid seeds grid with alive cells that are a live at a rate of 1 / aliveRate
// it returns a changeList of all the cells that changed
func seedGrid(grid [][]bool, aliveRate int) []cellDiff {
	changeList := make([]cellDiff, 0, 512)

	for rowNum, row := range grid {
		for colNum := range row {
			if rand.Intn(aliveRate) == 0 {
				grid[rowNum][colNum] = true

				changeList = append(changeList, cellDiff{row: rowNum, col: colNum})
			}
		}
	}

	return changeList
}

type gridDrawer struct {
	// this picData is reused between draw calls
	picData pixel.PictureData

	windowWidth  int
	windowHeight int

	// cells in the grid that we are drawing
	rows int
	cols int

	cellWidthPixels  int
	cellHeightPixels int

	pixelsToNextRow int
}

// initializes the pixData of the drawer, the dimension fields must have been set at this point
func (gd *gridDrawer) init() {
	monitorW, monitorH := pixelgl.PrimaryMonitor().Size()

	gd.picData.Pix = make([]color.RGBA, int(monitorW)*int(monitorH))

	fmt.Printf("total pixels: %d\n", len(gd.picData.Pix))

	gd.windowWidth = gd.cols * gd.cellWidthPixels
	gd.windowHeight = gd.rows * gd.cellHeightPixels

	fmt.Printf("window stride: %d\n", gd.windowWidth)

	gd.picData.Stride = gd.windowWidth

	gd.pixelsToNextRow = gd.windowWidth * gd.cellHeightPixels

	fmt.Printf("pixels to next row: %d\n", gd.pixelsToNextRow)

	gd.picData.Rect = pixel.Rect{
		Max: pixel.Vec{
			X: float64(gd.windowWidth),
			Y: float64(gd.windowHeight),
		},
	}
}

var totalDrawTime time.Duration
var drawCalls int

// draws all the changes in changeList to window
// drawing is done based on diffs because it saves a lot of iterations and the previous state stays drawn to the screen
// so only cells that changed need to have their color changed
func (gd *gridDrawer) drawDiff(gridRows [][]bool, changeList []cellDiff, window *pixelgl.Window) {
	drawCalls++
	defer func(start time.Time) {
		totalDrawTime += time.Since(start)
	}(time.Now())

	for _, change := range changeList {
		cellUpperLeftPixel := change.row*gd.pixelsToNextRow + change.col*gd.cellWidthPixels

		var cellColor color.RGBA
		if gridRows[change.row][change.col] {
			cellColor = colornames.Black
		} else {
			cellColor = colornames.White
		}

		for down := 0; down < gd.cellHeightPixels; down++ {
			downOffset := down * gd.windowWidth

			for right := 0; right < gd.cellWidthPixels; right++ {

				gd.picData.Pix[cellUpperLeftPixel+right+downOffset] = cellColor

			}
		}
	}

	sprite := pixel.NewSprite(&gd.picData, gd.picData.Bounds())

	sprite.Draw(window, pixel.IM.Moved(window.Bounds().Center()))
}

// recalculates gd's fields necessary to draw to window if the window's size is different from the gd's fields
//
func (gd *gridDrawer) maybeReconfigure(window *pixelgl.Window) bool {
	// potential resize
	bounds := window.Bounds()

	windowWidth := int(bounds.Max.X - bounds.Min.Y)
	windowHeight := int(bounds.Max.Y - bounds.Min.Y)

	var changed bool

	if gd.windowWidth != windowWidth {
		changed = true

		gd.pixelsToNextRow = windowWidth * gd.cellHeightPixels

		gd.windowWidth = windowWidth
		gd.picData.Stride = windowWidth

		gd.cellWidthPixels = windowWidth / gd.cols

		gd.picData.Rect.Max.X = float64(windowWidth)
	}

	if gd.windowHeight != windowHeight {
		changed = true

		gd.windowHeight = windowHeight

		gd.cellHeightPixels = windowHeight / gd.rows

		gd.pixelsToNextRow = gd.windowWidth * gd.cellHeightPixels

		gd.picData.Rect.Max.Y = float64(windowHeight)
	}

	return changed
}

// draws a full grid to window
func (gd *gridDrawer) drawGrid(grid [][]bool, window *pixelgl.Window) {
	for rowNum, row := range grid {
		for colNum, cell := range row {
			cellUpperLeftPixel := rowNum*gd.pixelsToNextRow + colNum*gd.cellWidthPixels

			var cellColor color.RGBA
			if cell {
				cellColor = colornames.Black
			} else {
				cellColor = colornames.White
			}

			for down := 0; down < gd.cellHeightPixels; down++ {
				downOffset := down * gd.windowWidth

				for right := 0; right < gd.cellWidthPixels; right++ {

					gd.picData.Pix[cellUpperLeftPixel+right+downOffset] = cellColor

				}
			}
		}
	}
}

var totalDoTurnTime time.Duration
var doTurnCalls int

// doTurn does a game of life tick based on state from and places the result into to
// returns the change list of cells that changed
func doTurn(from, to [][]bool) []cellDiff {
	doTurnCalls++
	defer func(start time.Time) {
		totalDoTurnTime += time.Since(start)
	}(time.Now())

	changeList := make([]cellDiff, 0, 512)

	lastRow := len(from) - 1
	lastCol := len(from[0]) - 1

	for rowNum, row := range from {
		for colNum, cell := range row {

			var aliveNeighbors int

			upRow := rowNum - 1
			if rowNum == 0 {
				upRow = lastRow
			}

			leftCol := colNum - 1
			if colNum == 0 {
				leftCol = lastCol
			}

			downRow := rowNum + 1
			if rowNum == lastRow {
				downRow = 0
			}

			rightCol := colNum + 1
			if colNum == lastCol {
				rightCol = 0
			}

			// up left
			if from[upRow][leftCol] {
				aliveNeighbors++
			}
			// up
			if from[upRow][colNum] {
				aliveNeighbors++
			}
			// up right
			if from[upRow][rightCol] {
				aliveNeighbors++
			}
			// left
			if from[rowNum][leftCol] {
				aliveNeighbors++
			}
			// right
			if from[rowNum][rightCol] {
				aliveNeighbors++
			}
			// down left
			if from[downRow][leftCol] {
				aliveNeighbors++
			}
			// down
			if from[downRow][colNum] {
				aliveNeighbors++
			}
			// down right
			if from[downRow][rightCol] {
				aliveNeighbors++
			}

			alive := aliveNeighbors == 3 || (cell && aliveNeighbors == 2)

			to[rowNum][colNum] = alive

			if alive != cell {
				changeList = append(changeList, cellDiff{row: rowNum, col: colNum})
			}
		}
	}

	return changeList
}

func applyChanges(grid [][]bool, changeList []cellDiff) {
	for _, change := range changeList {
		grid[change.row][change.col] = !grid[change.row][change.col]
	}
}

func initVars() (rows, cols, cellWidthPixels, cellHeightPixels, aliveRate int, tickRate time.Duration) {
	flag.IntVar(&rows, "rows", 100, "number of rows for the game of life")
	flag.IntVar(&cols, "cols", 100, "number of columns for the game of life")
	flag.IntVar(&cellWidthPixels, "cellWidthPixels", 10, "the height of a cell in pixels")
	flag.IntVar(&cellHeightPixels, "cellHeightPixels", 10, "the width of a cell in pixels")
	flag.IntVar(&aliveRate, "aliveRate", 3, "1 / aliveRate is the chance a cell is alive at start")
	flag.DurationVar(&tickRate, "tickRate", 100*time.Millisecond, "amount of time to take between ticks")

	flag.Parse()

	return rows, cols, cellWidthPixels, cellHeightPixels, aliveRate, tickRate
}
