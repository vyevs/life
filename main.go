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
	changeLists       [][]vec2
	lastChangeListIdx int
)

type vec2 struct {
	x int
	y int
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

	changeLists = make([][]vec2, 0, 512)

	lastChangeListIdx = -1

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
				X: float64(rows * cellWidthPixels),
				Y: float64(cols * cellHeightPixels),
			},
		},
		VSync: true,
		// Resizable: true,
	})

	if err != nil {
		log.Fatal(err)
	}

	ticker := time.Tick(tickRate)

	draw(window)

	window.Update()

	var lastPaused time.Time
	var lastRightPress time.Time
	var lastLeftPress time.Time

	for !window.Closed() {

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

		}

		select {
		case <-ticker:
			if !paused {
				doTurn()
			}

		default:

		}

		draw(window)

		window.Update()
	}

}

func seedGrid() {
	for i := range grid1 {
		grid1[i] = rand.Intn(3) == 1
	}
}

// don't make a new pixel.PictureData every draw
var picDataInit bool
var picData pixel.PictureData

func draw(window *pixelgl.Window) {
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
					X: float64(rows * cellWidthPixels),
					Y: float64(cols * cellHeightPixels),
				},
			},
		}
		picDataInit = true
	}

	for i, row := range grid1Rows {
		for j, cell := range row {

			cellUpperLeftPixel := i*stridePixels + j*cellWidthPixels

			// move down and right from the cell's upper left pixel
			for down := 0; down < cellHeightPixels; down++ {
				downOffset := down * windowWidthPixels

				for right := 0; right < cellWidthPixels; right++ {

					var cellColor color.RGBA
					if cell {
						cellColor = colornames.Black
					} else {
						cellColor = colornames.White
					}

					picData.Pix[cellUpperLeftPixel+right+downOffset] = cellColor

				}
			}
		}
	}

	sprite := pixel.NewSprite(&picData, picData.Bounds())

	sprite.Draw(window, pixel.IM.Moved(window.Bounds().Center()))
}

func doTurn() {
	defer func(start time.Time) {
		fmt.Printf("doTurn took %s\n", time.Since(start))
	}(time.Now())

	// try to apply already saved changes to go forward
	if lastChangeListIdx < len(changeLists)-1 {

		// increment first because we want the changes to get us to the next state
		// since lastChangeListIdx contains the changes that got us to the current state
		lastChangeListIdx++

		applyChange(grid1Rows, changeLists[lastChangeListIdx])

		return
	}

	changeList := make([]vec2, 0, 512)

	for i, row := range grid1Rows {
		for j, cell := range row {

			var aliveNeighbors int

			//fmt.Printf("cell (%d, %d)\n", i, j)

			for rowOffset := -1; rowOffset <= 1; rowOffset++ {
				for colOffset := -1; colOffset <= 1; colOffset++ {

					if rowOffset == 0 && colOffset == 0 {
						continue
					}

					neighborRow := i + rowOffset + rows

					if neighborRow >= rows {
						neighborRow -= rows
						if neighborRow >= rows {
							neighborRow -= rows
						}
					}

					neighborCol := j + colOffset + cols

					for neighborCol >= cols {
						neighborCol -= cols
						for neighborCol >= cols {
							neighborCol -= cols
						}
					}

					//fmt.Printf("neighbor (%d, %d)\n", neighborRow, neighborCol)

					if grid1Rows[neighborRow][neighborCol] {
						aliveNeighbors++
					}
				}
			}

			grid2Rows[i][j] = aliveNeighbors == 3 || (cell && aliveNeighbors == 2)

			if grid2Rows[i][j] != cell {
				changeList = append(changeList, vec2{x: i, y: j})
			}
		}
	}

	fmt.Printf("changes: %d\n", len(changeList))

	changeLists = append(changeLists, changeList)
	lastChangeListIdx++

	grid1, grid2 = grid2, grid1
	grid1Rows, grid2Rows = grid2Rows, grid1Rows
}

func doTurn2() {
	defer func(start time.Time) {
		fmt.Printf("doTurn2 took %s\n", time.Since(start))
	}(time.Now())

	// try to apply already saved changes to go forward
	if lastChangeListIdx < len(changeLists)-1 {

		// increment first because we want the changes to get us to the next state
		// since lastChangeListIdx contains the changes that got us to the current state
		lastChangeListIdx++

		applyChange(grid1Rows, changeLists[lastChangeListIdx])

		return
	}

	changeList := make([]vec2, 0, 512)

	// keep a running track of alive neighbors as we slide the window
	//var aliveNeighbors int

	for i, row := range grid1Rows {

		for j, cell := range row {

			var aliveNeighbors int

			for rowOffset := -1; rowOffset <= 1; rowOffset++ {
				for colOffset := -1; colOffset <= 1; colOffset++ {

					if rowOffset == 0 && colOffset == 0 {
						continue
					}

					neighborRow := i + rowOffset
					neighborCol := j + colOffset

					if neighborRow < 0 {
						neighborRow = rows - 1
					} else if neighborRow >= rows {
						neighborRow = 0
					}

					if neighborCol < 0 {
						neighborCol = cols - 1
					} else if neighborCol >= cols {
						neighborCol = 0
					}

					if grid1Rows[neighborRow][neighborCol] {
						aliveNeighbors++
					}
				}
			}

			grid2Rows[i][j] = aliveNeighbors == 3 || (cell && aliveNeighbors == 2)

			if grid2Rows[i][j] != cell {
				changeList = append(changeList, vec2{x: i, y: j})
			}
		}
	}

	fmt.Printf("changes: %d\n", len(changeList))

	changeLists = append(changeLists, changeList)
	lastChangeListIdx++

	grid1, grid2 = grid2, grid1
	grid1Rows, grid2Rows = grid2Rows, grid1Rows
}

func reverseChange() {
	if lastChangeListIdx == -1 {
		return
	}

	applyChange(grid1Rows, changeLists[lastChangeListIdx])

	lastChangeListIdx--
}

func applyChange(gridRows [][]bool, changeList []vec2) {
	for _, change := range changeList {
		gridRows[change.x][change.y] = !gridRows[change.x][change.y]
	}
}

func readArgs() {
	flag.IntVar(&rows, "rows", 100, "number of rows for the game of life")
	flag.IntVar(&cols, "cols", 100, "number of columns for the game of life")
	flag.IntVar(&cellWidthPixels, "cellWidthPixels", 10, "the height of a cell in pixels")
	flag.IntVar(&cellHeightPixels, "cellHeightPixels", 10, "the width of a cell in pixels")
	flag.DurationVar(&tickRate, "tickRate", 100*time.Millisecond, "amount of time to take between ticks")

	flag.Parse()
}
