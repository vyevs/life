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

	rand.Seed(time.Now().UnixNano())

	var grid1 []bool

	var rows, cols int

	var seedFile string

	var tickRate time.Duration

	flag.IntVar(&rows, "r", 50, "number of rows for the game of life")
	flag.IntVar(&cols, "c", 180, "number of columns for the game of life")
	flag.StringVar(&seedFile, "sf", "", "seed file to load seed from")
	flag.DurationVar(&tickRate, "t", 100*time.Millisecond, "amount of time to take between ticks")

	fmt.Println(rows, cols, tickRate)

	flag.Parse()

	grid1 = make([]bool, rows*cols)

	grid2 := make([]bool, rows*cols)

	latest, other := grid1, grid2

	ticker := time.Tick(tickRate)

	drawGrid(latest, rows, cols)

	for range ticker {
		doTurn(latest, other, rows, cols)
		//os.Exit(1)
		drawGrid(latest, rows, cols)
		latest, other = other, latest
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
	buf.WriteRune('\n')

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
