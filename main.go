package main

import (
	"flag"
	"fmt"
	"image/color"
	"log"
	"math/rand"
	"time"

	"github.com/faiface/pixel"
	"github.com/faiface/pixel/pixelgl"
	"golang.org/x/image/colornames"
)

func main() {
	pixelgl.Run(run)
}

var (
	// swap buffers each frame, buf1 is always the one to display
	grid1 []bool
	grid2 []bool

	// 2d views of the grids, for simpler manipulation
	grid1Rows [][]bool
	grid2Rows [][]bool

	rows int
	cols int

	cellWidthPixels  int
	cellHeightPixels int

	windowWidthPixels  int
	windowHeightPixels int

	// number of pixels in flat pixel view to move right to get to the cell below you
	stridePixels int

	tickRate time.Duration

	paused bool

	// changeLists[i] contains the cell positions that changed state in moving from step i -> i + 1
	changeLists       [][]cellLoc
	lastChangeListIdx = -1

	// change of a cell being alive from the start is 1 / aliveRate
	aliveRate int

	reversed bool
)

type cellLoc struct {
	row int
	col int
}

func run() {

	readArgs()

	windowWidthPixels = cols * cellWidthPixels
	windowHeightPixels = rows * cellHeightPixels

	stridePixels = windowWidthPixels * cellHeightPixels

	grid1 = make([]bool, rows*cols)
	grid2 = make([]bool, rows*cols)

	grid1Rows = make([][]bool, rows)
	grid2Rows = make([][]bool, rows)

	for i := 0; i < rows; i++ {
		rowStart := i * cols

		grid1Rows[i] = grid1[rowStart : rowStart+cols]
		grid2Rows[i] = grid2[rowStart : rowStart+cols]
	}

	changeLists = make([][]cellLoc, 0, 512)

	rand.Seed(time.Now().UnixNano())

	seedGrid()

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
		// Resizable: true,
	})

	if err != nil {
		log.Fatal(err)
	}

	ticker := time.Tick(tickRate)

	fpsTicker := time.Tick(1 * time.Second)
	var frames int

	window.Clear(colornames.White)

	window.Update()

	var lastPaused time.Time
	var lastRightPress time.Time
	var lastLeftPress time.Time
	var lastShiftPress time.Time

	for !window.Closed() {

		reversed = false

		frames++

		if window.Pressed(pixelgl.KeySpace) && time.Since(lastPaused) >= tickRate {
			lastPaused = time.Now()
			paused = !paused
		}

		if window.Pressed(pixelgl.KeyRight) && paused && time.Since(lastRightPress) >= tickRate {
			lastRightPress = time.Now()

			doTurn()
		}

		if window.Pressed(pixelgl.KeyLeft) && paused && time.Since(lastLeftPress) >= tickRate {
			lastLeftPress = time.Now()

			reverseChange()

			reversed = true
		}

		if window.Pressed(pixelgl.KeyLeftShift) && time.Since(lastShiftPress) >= tickRate {
			lastShiftPress = time.Now()

			lastChangeListIdx = -1

			changeLists = changeLists[:0]

			seedGrid()
		}

		select {
		case <-ticker:
			if !paused {
				doTurn()
			}
		default:
		}

		select {
		case <-fpsTicker:
			window.SetTitle(fmt.Sprintf("FPS: %d", frames))
			frames = 0
		default:
		}

		draw(window)

		window.Update()
	}

	fmt.Printf("average do turn time: %s\n", totalDoTurnTime/time.Duration(doTurnCalls))
	fmt.Printf("average draw time: %s\n", totalDrawTime/time.Duration(drawCalls))
}

func seedGrid() {
	changeList := make([]cellLoc, 0, 512)

	for i, row := range grid1Rows {
		for j := range row {
			alive := rand.Intn(aliveRate) == 0

			if alive {
				grid1Rows[i][j] = alive

				changeList = append(changeList, cellLoc{row: i, col: j})
			}
		}
	}

	lastChangeListIdx++

	changeLists = append(changeLists, changeList)
}

// don't make a new pixel.PictureData every draw
var picDataInit bool
var picData pixel.PictureData

var totalDrawTime time.Duration
var drawCalls int

func draw(window *pixelgl.Window) {
	drawCalls++
	defer func(start time.Time) {
		totalDrawTime += time.Since(start)
	}(time.Now())

	if !picDataInit {
		picData = pixel.PictureData{
			Pix:    make([]color.RGBA, rows*cellWidthPixels*cols*cellHeightPixels),
			Stride: cols * cellWidthPixels,
			Rect: pixel.Rect{
				Min: pixel.Vec{
					X: 0,
					Y: 0,
				},
				Max: pixel.Vec{
					X: float64(cols * cellWidthPixels),
					Y: float64(rows * cellHeightPixels),
				},
			},
		}
		picDataInit = true
	}

	var changeList []cellLoc
	if reversed {
		changeList = changeLists[lastChangeListIdx+1]
	} else {
		changeList = changeLists[lastChangeListIdx]
	}

	for _, change := range changeList {
		cellUpperLeftPixel := change.row*stridePixels + change.col*cellWidthPixels

		var cellColor color.RGBA
		if grid1Rows[change.row][change.col] {
			cellColor = colornames.Black
		} else {
			cellColor = colornames.White
		}

		for down := 0; down < cellHeightPixels; down++ {
			downOffset := down * windowWidthPixels

			for right := 0; right < cellWidthPixels; right++ {

				picData.Pix[cellUpperLeftPixel+right+downOffset] = cellColor

			}
		}
	}

	sprite := pixel.NewSprite(&picData, picData.Bounds())

	sprite.Draw(window, pixel.IM.Moved(window.Bounds().Center()))
}

var totalDoTurnTime time.Duration
var doTurnCalls int

func doTurn() {
	doTurnCalls++
	defer func(start time.Time) {
		totalDoTurnTime += time.Since(start)
	}(time.Now())

	// try to apply already saved changes to go forward
	if lastChangeListIdx < len(changeLists)-1 {
		lastChangeListIdx++

		applyChange(grid1Rows, changeLists[lastChangeListIdx])

		return
	}

	changeList := make([]cellLoc, 0, 512)

	for i, row := range grid1Rows {

		for j, cell := range row {

			var aliveNeighbors int

			if i == 0 {
				wrappedRow := rows - 1

				// up & left
				if j == 0 {
					wrappedCol := cols - 1

					if grid1[wrappedRow*cols+wrappedCol] {
						aliveNeighbors++
					}
				} else if grid1[wrappedRow*cols+j-1] {
					aliveNeighbors++
				}

				// up
				if grid1[wrappedRow*cols+j] {
					aliveNeighbors++
				}

				// up & right
				if j == cols-1 {
					wrappedCol := 0

					if grid1[wrappedRow*cols+wrappedCol] {
						aliveNeighbors++
					}
				} else if grid1[wrappedRow*cols+j+1] {
					aliveNeighbors++
				}

			} else {

				// up & left
				if j == 0 {
					wrappedJ := cols - 1

					if grid1[(i-1)*cols+wrappedJ] {
						aliveNeighbors++
					}
				} else if grid1[(i-1)*cols+j-1] {
					aliveNeighbors++
				}

				// up
				if grid1[(i-1)*cols+j] {
					aliveNeighbors++
				}

				// up & right
				if j == cols-1 {
					wrappedJ := 0

					if grid1[(i-1)*cols+wrappedJ] {
						aliveNeighbors++
					}
				} else if grid1[(i-1)*cols+j+1] {
					aliveNeighbors++
				}
			}

			if i == rows-1 {
				wrappedRow := 0

				// down & left
				if j == 0 {
					wrappedCol := cols - 1

					if grid1[wrappedRow*cols+wrappedCol] {
						aliveNeighbors++
					}
				} else if grid1[wrappedRow*cols+j-1] {
					aliveNeighbors++
				}

				// down
				if grid1[wrappedRow*cols+j] {
					aliveNeighbors++
				}

				// down & right
				if j == cols-1 {
					wrappedCol := 0

					if grid1[wrappedRow*cols+wrappedCol] {
						aliveNeighbors++
					}
				} else if grid1[wrappedRow*cols+j+1] {
					aliveNeighbors++
				}

			} else {

				// down & left
				if j == 0 {
					wrappedCol := cols - 1

					if grid1[(i+1)*cols+wrappedCol] {
						aliveNeighbors++
					}
				} else if grid1[(i+1)*cols+j-1] {
					aliveNeighbors++
				}

				// down
				if grid1[(i+1)*cols+j] {
					aliveNeighbors++
				}

				// down & right
				if j == cols-1 {
					wrappedCol := 0

					if grid1[(i+1)*cols+wrappedCol] {
						aliveNeighbors++
					}
				} else if grid1[(i+1)*cols+j+1] {
					aliveNeighbors++
				}
			}

			// left, wrap to last column
			if j == 0 {
				wrappedCol := cols - 1

				if grid1[i*cols+wrappedCol] {
					aliveNeighbors++
				}

			} else if grid1[i*cols+j-1] { // left, no column wrap
				aliveNeighbors++
			}

			// right, wrap to 1st column
			if j == cols-1 {
				wrappedCol := 0

				if grid1[i*cols+wrappedCol] {
					aliveNeighbors++
				}
			} else if grid1[i*cols+j+1] { // right, no column wrap
				aliveNeighbors++
			}

			grid2Rows[i][j] = aliveNeighbors == 3 || (cell && aliveNeighbors == 2)

			if grid2Rows[i][j] != cell {
				changeList = append(changeList, cellLoc{row: i, col: j})
			}
		}
	}

	changeLists = append(changeLists, changeList)

	lastChangeListIdx++

	grid1, grid2 = grid2, grid1
	grid1Rows, grid2Rows = grid2Rows, grid1Rows
}

// moves the grid1 state into the state prior to the current grid1 state
// only goes as far as the initial seed state, not to empty grid
func reverseChange() {
	if lastChangeListIdx == 0 {
		return
	}

	applyChange(grid1Rows, changeLists[lastChangeListIdx])

	lastChangeListIdx--
}

func applyChange(gridRows [][]bool, changeList []cellLoc) {
	for _, change := range changeList {
		gridRows[change.row][change.col] = !gridRows[change.row][change.col]
	}
}

func readArgs() {
	flag.IntVar(&rows, "rows", 100, "number of rows for the game of life")
	flag.IntVar(&cols, "cols", 100, "number of columns for the game of life")
	flag.IntVar(&cellWidthPixels, "cellWidthPixels", 10, "the height of a cell in pixels")
	flag.IntVar(&cellHeightPixels, "cellHeightPixels", 10, "the width of a cell in pixels")
	flag.IntVar(&aliveRate, "aliveRate", 3, "1 / aliveRate is the chance a cell is alive at start")
	flag.DurationVar(&tickRate, "tickRate", 100*time.Millisecond, "amount of time to take between ticks")

	flag.Parse()
}
