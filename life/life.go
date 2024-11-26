package life

type board struct {
	current [][]bool
	next    [][]bool
	final   bool
}

// NextGeneration returns the next generation of the input board, after applying Conway's rules to
// each cell. final is true if no cells are alive in the next generation.
func NextGeneration(input [][]bool) ([][]bool, bool) {
	b := &board{
		current: input,
		next:    make([][]bool, len(input)),
		final:   true,
	}

	for i := range b.next {
		b.next[i] = make([]bool, len(input[i]))
	}

	b.nextGeneration()

	return b.next, b.final
}

func (b *board) nextGeneration() {
	for i := range b.current {
		for j := range b.current[i] {
			alive := b.current[i][j]        // is current cell alive?
			count := b.countNeighbors(i, j) // how many living neighbors does it have?

			switch count {
			case 3:
				// whether dead or alive, cell becomes alive if it has exactly 3 living neighbors
				b.next[i][j] = true
			case 2:
				// if alive, cell stays alive if it has 2 living neighbors
				b.next[i][j] = alive
			default:
				// all other cases, cell should be dead. Let zero value of bool remain.
			}

			// board is not final if any cell is alive
			if b.next[i][j] {
				b.final = false
			}
		}
	}
}

func (b *board) countNeighbors(x, y int) int {
	count := 0

	for i := x - 1; i <= x+1; i++ {
		for j := y - 1; j <= y+1; j++ {
			// skip the current cell
			if i == x && j == y {
				continue
			}

			// don't access out of slice bounds
			if i < 0 || i >= len(b.current) {
				continue
			}

			if j < 0 || j >= len(b.current[i]) {
				continue
			}

			if b.current[i][j] {
				count++
			}
		}
	}

	return count
}
