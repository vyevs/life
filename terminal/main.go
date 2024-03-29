package main

import (
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strings"
	"time"
)

func main() {

	var grid1 []bool

	var rows, cols int

	var seedFile string

	var tickRate time.Duration

	flag.IntVar(&rows, "r", 50, "number of rows for the game of life")
	flag.IntVar(&cols, "c", 100, "number of columns for the game of life")
	flag.StringVar(&seedFile, "sf", "", "seed file to load seed from")
	flag.DurationVar(&tickRate, "t", 100*time.Millisecond, "amount of time to take between ticks")

	flag.Parse()

	if seedFile != "" {
		grid1, rows, cols = loadSeed(seedFile)
		if grid1 == nil {
			fmt.Printf("unable to load seed from file %s\n", seedFile)
		}
	}

	if grid1 == nil {
		rand.Seed(time.Now().UnixNano())
		grid1 = randSeed(rows, cols)
	}

	fmt.Println(rows, cols, tickRate)

	grid2 := make([]bool, rows*cols)

	latest, other := grid1, grid2

	ticker := time.Tick(tickRate)

	drawGrid(latest, rows, cols)

	for range ticker {
		doTurn(latest, other, rows, cols)
		latest, other = other, latest
		drawGrid(latest, rows, cols)
	}

}

func randSeed(rows, cols int) []bool {
	out := make([]bool, rows*cols)

	for i := range out {
		out[i] = rand.Intn(3) == 1
	}

	return out
}

func drawGrid(grid []bool, rows, cols int) {
	var buf strings.Builder
	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			pos := i*cols + j

			if grid[pos] {
				buf.WriteRune('█')
			} else {
				buf.WriteRune(' ')
			}
		}
		buf.WriteRune('\n')
	}
	buf.WriteString("\n\n")

	fmt.Println(buf.String())
}

func doTurn(from, to []bool, rows, cols int) {
	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			var aliveNeighbors int

			if i == 0 {
				wrappedRow := rows - 1

				// up & left
				if j == 0 {
					wrappedCol := cols - 1

					if from[wrappedRow*cols+wrappedCol] {
						aliveNeighbors++
					}
				} else if from[wrappedRow*cols+j-1] {
					aliveNeighbors++
				}

				// up
				if from[wrappedRow*cols+j] {
					aliveNeighbors++
				}

				// up & right
				if j == cols-1 {
					wrappedCol := 0

					if from[wrappedRow*cols+wrappedCol] {
						aliveNeighbors++
					}
				} else if from[wrappedRow*cols+j+1] {
					aliveNeighbors++
				}

			} else {

				// up & left
				if j == 0 {
					wrappedJ := cols - 1

					if from[(i-1)*cols+wrappedJ] {
						aliveNeighbors++
					}
				} else if from[(i-1)*cols+j-1] {
					aliveNeighbors++
				}

				// up
				if from[(i-1)*cols+j] {
					aliveNeighbors++
				}

				// up & right
				if j == cols-1 {
					wrappedJ := 0

					if from[(i-1)*cols+wrappedJ] {
						aliveNeighbors++
					}
				} else if from[(i-1)*cols+j+1] {
					aliveNeighbors++
				}
			}

			if i == rows-1 {
				wrappedRow := 0

				// down & left
				if j == 0 {
					wrappedCol := cols - 1

					if from[wrappedRow*cols+wrappedCol] {
						aliveNeighbors++
					}
				} else if from[wrappedRow*cols+j-1] {
					aliveNeighbors++
				}

				// down
				if from[wrappedRow*cols+j] {
					aliveNeighbors++
				}

				// down & right
				if j == cols-1 {
					wrappedCol := 0

					if from[wrappedRow*cols+wrappedCol] {
						aliveNeighbors++
					}
				} else if from[wrappedRow*cols+j+1] {
					aliveNeighbors++
				}

			} else {

				// down & left
				if j == 0 {
					wrappedCol := cols - 1

					if from[(i+1)*cols+wrappedCol] {
						aliveNeighbors++
					}
				} else if from[(i+1)*cols+j-1] {
					aliveNeighbors++
				}

				// down
				if from[(i+1)*cols+j] {
					aliveNeighbors++
				}

				// down & right
				if j == cols-1 {
					wrappedCol := 0

					if from[(i+1)*cols+wrappedCol] {
						aliveNeighbors++
					}
				} else if from[(i+1)*cols+j+1] {
					aliveNeighbors++
				}
			}

			// left, wrap to last column
			if j == 0 {
				wrappedCol := cols - 1

				if from[i*cols+wrappedCol] {
					aliveNeighbors++
				}

			} else if from[i*cols+j-1] { // left, no column wrap
				aliveNeighbors++
			}

			// right, wrap to 1st column
			if j == cols-1 {
				wrappedCol := 0

				if from[i*cols+wrappedCol] {
					aliveNeighbors++
				}
			} else if from[i*cols+j+1] { // right, no column wrap
				aliveNeighbors++
			}

			//fmt.Printf("(%d, %d) has %d alive neighbors\n", i, j, aliveNeighbors)

			pos := i*cols + j

			if from[pos] {
				if aliveNeighbors == 2 || aliveNeighbors == 3 {
					to[pos] = true
				} else {
					to[pos] = false
				}
			} else if aliveNeighbors == 3 {
				to[pos] = true
			} else {
				to[pos] = false
			}
		}
	}
}

// EXPERIMENTAL for speedup, might not actually be that much faster than original
// writes the state resulting from from into to
// stride is the number of columns each row has
func doTurn2(from, to []bool, stride int) {
	for pos := range from {
		var aliveNeighbors int

		// the following nested branches are ordered in most likely -> least likely
		// on a 100x100 grid, 9602/10000 cells will not go inside the outer branches
		// e.g.: most cells have have a direct up and left neighbor, only cells on the top and left borders do not, for which we wrap around

		var upLeft int

		//  try directly up and left
		if upLeft = pos - stride - 1; upLeft < 0 {

			// try directly up and wrapping to last col
			if upLeft = pos - 1; upLeft < 0 {

				// this cell is in the upper left corner, wrap to last row and col
				upLeft = len(from) - 1
			}
		}

		if from[upLeft] {
			aliveNeighbors++
		}

		var up int

		// try directly up
		if up = pos - stride; up < 0 {

			// this cell is in the top row, wrap to last row
			up = len(from) - (stride - pos)
		}

		if from[up] {
			aliveNeighbors++
		}

		var upRight int

		// try directly up and right
		if upRight = pos - stride + 1; upRight < 0 {

			// try directly up and wrapping to first col
			if upRight = pos - stride - stride + 1; upRight < 0 {

				// this cell is in the upper right corner, wrap to last row & first col
				upRight = len(from) - stride
			}
		}

		if from[upRight] {
			aliveNeighbors++
		}

		// go directly left
		left := pos - 1
		// cell is on the left border and pos-1 wrapped around, so move down 1 row
		if pos%stride == 0 {
			left += stride
		}

		if from[left] {
			aliveNeighbors++
		}

		// go directly right
		right := pos + 1
		// cell is n the right border and pos+1 wrapped around, so move up 1 row
		if pos%stride == stride-1 {
			right -= stride
		}

		if from[right] {
			aliveNeighbors++
		}

		var downLeft int

		// try directly down and left
		if downLeft = pos + stride - 1; downLeft >= len(from) {

			// try wrapping to last col
			if downLeft = pos + stride + stride - 1; downLeft >= len(from) {

				// we are in bottom left corner
				downLeft = stride - 1
			}
		}

		if from[downLeft] {
			aliveNeighbors++
		}

		var down int

		if down = pos + stride; down >= len(from) {

			// we are in bottom row, wrap to top
			down = stride - (len(from) - pos)
		}

		if from[down] {
			aliveNeighbors++
		}

		var downRight int

		// try directly down and right
		if downRight = pos + stride + 1; downRight >= len(from) {

			// we are on last col
			if downRight = pos + 1; downRight >= len(from) {

				downRight = 0
			}
		}

		if from[downRight] {
			aliveNeighbors++
		}

		to[pos] = (from[pos] && (aliveNeighbors == 2 || aliveNeighbors == 3)) || aliveNeighbors == 3
	}
}

func loadSeed(path string) ([]bool, int, int) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, 0
	}
	defer f.Close()

	var rows, cols int
	_, err = fmt.Fscanln(f, &rows, &cols)
	if err != nil {
		return nil, 0, 0
	}

	out := make([]bool, rows*cols)

	for {
		var x, y int
		_, err = fmt.Fscanln(f, &x, &y)
		if err != nil {
			break
		}

		out[x*cols+y] = true
	}

	if err != io.EOF {
		return nil, 0, 0
	}

	return out, rows, cols
}
