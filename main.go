package main

import (
	"flag"
	"fmt"
	"image/color"
	"log"
	"math/rand"
	"strings"
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
	row      int
	col      int
	newState bool
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

	grid := make([][]bool, rows)

	for i := 0; i < rows; i++ {
		grid[i] = make([]bool, cols)
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

	seedGrid(grid, aliveRate)

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

	for !window.Closed() {

		// handle possible resizing of window
		gridDrawer.reconfigure(window)

		if window.JustPressed(pixelgl.KeySpace) {
			paused = !paused

			if paused {
				fmt.Println("paused")
			} else {
				fmt.Println("unpaused")
			}

		} else if window.JustPressed(pixelgl.KeyRight) && paused {
			// can't reuse a saved diff, make a move and then use those changes
			if lastChangesIdx == len(changeLists)-1 {
				newGrid := doTurn(grid)

				changes := determineChanges(grid, newGrid)

				grid = newGrid

				changeLists = append(changeLists, changes)
				lastChangesIdx++

			} else { // can reuse a saved diff here

				lastChangesIdx++

				change := changeLists[lastChangesIdx]

				applyChanges(grid, change)
			}

		} else if window.JustPressed(pixelgl.KeyLeft) && paused {

			// apply the diffs at lastChangeIdx to the grid & then draw using that grid & diffs
			if lastChangesIdx >= 0 {
				changes := changeLists[lastChangesIdx]

				applyChanges(grid, changes)
				lastChangesIdx--
			}

		} else if window.JustPressed(pixelgl.KeyLeftShift) {

			changeLists = changeLists[:0]

			newInitialChanges := seedGrid(grid, aliveRate)

			changeLists = append(changeLists, newInitialChanges)

		} else if window.JustPressed(pixelgl.KeyComma) {
			ticker.Stop()
			for range ticker.C {
			}
			tickRate *= 2
			fmt.Printf("slowing down, new tickRate: %s\n", tickRate)
			ticker = time.NewTicker(tickRate)
		} else if window.JustPressed(pixelgl.KeyPeriod) {
			ticker.Stop()
			for range ticker.C {
			}
			tickRate /= 2
			fmt.Printf("speeding up, new tickRate: %s\n", tickRate)
			ticker = time.NewTicker(tickRate)
		}

		select {
		case <-ticker.C:
			if !paused {
				newGrid := doTurn(grid)

				changes := determineChanges(grid, newGrid)

				grid = newGrid

				changeLists = append(changeLists, changes)
				lastChangesIdx++
			}
		default:
		}

		gridDrawer.drawGrid(grid, window)

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

	gridWidthPixels  int
	gridHeightPixels int

	pixelsToNextRow int
}

func (gd *gridDrawer) String() string {
	var buf strings.Builder

	buf.WriteString(fmt.Sprintf("window width: %d\n", gd.windowWidth))
	buf.WriteString(fmt.Sprintf("window height: %d\n", gd.windowHeight))
	buf.WriteString(fmt.Sprintf("rows: %d\n", gd.rows))
	buf.WriteString(fmt.Sprintf("cols: %d\n", gd.cols))
	buf.WriteString(fmt.Sprintf("cellWidthPixels: %d\n", gd.cellWidthPixels))
	buf.WriteString(fmt.Sprintf("cellHeightPixels: %d\n", gd.cellHeightPixels))
	buf.WriteString(fmt.Sprintf("pixelsToNextRow: %d\n", gd.pixelsToNextRow))

	return buf.String()
}

// initializes the pixData of the drawer, the dimension fields must have been set at this point
func (gd *gridDrawer) init() {
	monitorW, monitorH := pixelgl.PrimaryMonitor().Size()

	gd.picData.Pix = make([]color.RGBA, int(monitorW)*int(monitorH))

	fmt.Printf("total pixels: %d\n", len(gd.picData.Pix))

	gd.windowWidth = gd.cols * gd.cellWidthPixels
	gd.gridWidthPixels = gd.windowWidth
	gd.windowHeight = gd.rows * gd.cellHeightPixels
	gd.gridHeightPixels = gd.windowHeight

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

// recalculates gd's fields necessary to draw to window if the window's size is different from the gd's fields
func (gd *gridDrawer) reconfigure(window *pixelgl.Window) bool {
	// potential resize
	bounds := window.Bounds()

	windowWidth := int(bounds.Max.X - bounds.Min.X)
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

var totalDrawTime time.Duration
var drawCalls int

// draws a full grid to window
func (gd *gridDrawer) drawGrid(grid [][]bool, window *pixelgl.Window) {
	drawCalls++
	defer func(start time.Time) {
		totalDrawTime += time.Since(start)
	}(time.Now())

	for rowNum, row := range grid {
		for colNum, cell := range row {
			cellUpperLeftPixel := rowNum*gd.pixelsToNextRow + colNum*gd.cellWidthPixels

			var cellColor color.RGBA
			if cell {
				cellColor = colornames.Black
			} else {
				cellColor = colornames.White
			}

			for up := 0; up < gd.cellHeightPixels; up++ {
				upOffset := up * gd.windowWidth

				for right := 0; right < gd.cellWidthPixels; right++ {

					pixelLoc := cellUpperLeftPixel + right + upOffset

					gd.picData.Pix[pixelLoc] = cellColor

				}

			}

		}
	}

	sprite := pixel.NewSprite(&gd.picData, gd.picData.Bounds())

	sprite.Draw(window, pixel.IM.Moved(window.Bounds().Center()))
}

var totalDoTurnTime time.Duration
var doTurnCalls int

// doTurn does a game of life tick from the grid and returns the new state
func doTurn(grid [][]bool) [][]bool {
	doTurnCalls++
	defer func(start time.Time) {
		totalDoTurnTime += time.Since(start)
	}(time.Now())

	nextState := make([][]bool, len(grid))
	for i := 0; i < len(nextState); i++ {
		nextState[i] = make([]bool, len(grid[i]))
	}

	lastRow := len(grid) - 1
	lastCol := len(grid[0]) - 1

	for rowNum, row := range grid {
		for colNum, cell := range row {

			var aliveNeighbors int

			upRow := rowNum + 1
			if rowNum == lastRow {
				upRow = 0
			}

			leftCol := colNum - 1
			if colNum == 0 {
				leftCol = lastCol
			}

			downRow := rowNum - 1
			if rowNum == 0 {
				downRow = lastRow
			}

			rightCol := colNum + 1
			if colNum == lastCol {
				rightCol = 0
			}

			// up left
			if grid[upRow][leftCol] {
				aliveNeighbors++
			}
			// up
			if grid[upRow][colNum] {
				aliveNeighbors++
			}
			// up right
			if grid[upRow][rightCol] {
				aliveNeighbors++
			}
			// left
			if grid[rowNum][leftCol] {
				aliveNeighbors++
			}
			// right
			if grid[rowNum][rightCol] {
				aliveNeighbors++
			}
			// down left
			if grid[downRow][leftCol] {
				aliveNeighbors++
			}
			// down
			if grid[downRow][colNum] {
				aliveNeighbors++
			}
			// down right
			if grid[downRow][rightCol] {
				aliveNeighbors++
			}

			nextState[rowNum][colNum] = aliveNeighbors == 3 || (cell && aliveNeighbors == 2)
		}
	}

	return nextState
}

func determineChanges(state1, state2 [][]bool) []cellDiff {
	changes := make([]cellDiff, 0, 512)

	for row := 0; row < len(state1); row++ {
		for col := 0; col < len(state1[row]); col++ {
			if state2[row][col] != state1[row][col] {
				changes = append(changes, cellDiff{row: row, col: col, newState: state2[row][col]})
			}
		}
	}

	return changes
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
