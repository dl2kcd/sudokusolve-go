package main

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"time"
)

const rootNum = 3
const sideLen = rootNum * rootNum
const boardSize = sideLen * sideLen

type digitT uint8
type boardT [boardSize]digitT

var groups [3 * sideLen][sideLen]int
var groupsOfCell [boardSize][3]int

const (
	ROW = 0
	COL = 1
	BOX = 2
)

type optionsT struct {
	cell int
	n    int
	v    [sideLen]digitT
}

func make_groups() {
	var g int
	// rows
	for y := 0; y < sideLen; y++ {
		for x := 0; x < sideLen; x++ {
			cell := x + sideLen*y
			groups[g][x] = cell
			groupsOfCell[cell][ROW] = g
		}
		g++
	}
	// columns
	for x := 0; x < sideLen; x++ {
		for y := 0; y < sideLen; y++ {
			cell := x + sideLen*y
			groups[g][y] = cell
			groupsOfCell[cell][COL] = g
		}
		g++
	}
	// boxes
	for x1 := 0; x1 < rootNum; x1++ {
		for y1 := 0; y1 < rootNum; y1++ {
			var k int
			for x2 := 0; x2 < rootNum; x2++ {
				for y2 := 0; y2 < rootNum; y2++ {
					x := rootNum*x1 + x2
					y := rootNum*y1 + y2
					cell := x + sideLen*y
					groups[g][k] = cell
					groupsOfCell[cell][BOX] = g
					k++
				}
			}
			g++
		}
	}
	if g != len(groups) {
		panic("Programmer's errer!")
	}
}

func allowed(board *boardT, cell int, digit digitT) bool {
	for rcb := 0; rcb < 3; rcb++ {
		g := &groups[groupsOfCell[cell][rcb]]
		for i := 0; i < sideLen; i++ {
			if board[g[i]] == digit {
				return false
			}
		}
	}
	return true
}

func fillCellOptions(opt *optionsT, board *boardT, cell int) int {
	opt.cell = cell
	n := 0
	for x := digitT(1); x <= sideLen; x++ {
		if allowed(board, cell, x) {
			opt.v[n] = x
			n++
		}
	}
	opt.n = n
	return n
}

var solchan chan boardT
var wgout sync.WaitGroup
var wgsol sync.WaitGroup
var extragoro int

func solve(board boardT) {
	defer wgsol.Done()
	var emptCount int
	var modified bool = false
	var opt optionsT
	bestOpt := optionsT{n: sideLen + 1}

	for i := 0; i < boardSize; i++ {
		if board[i] != 0 {
			continue
		}
		emptCount++
		n := fillCellOptions(&opt, &board, i)
		if n == 0 { // hit empty cell with no options
			return
		}
		if n == 1 { // only one option, go for it
			board[i] = opt.v[0]
			emptCount--
			modified = true // some options at earlier minOptIdx may now be invalid
			continue
		}
		if n < bestOpt.n { // found cell with less options than old best
			bestOpt = opt
			modified = false // bestOpt matches current board
		}
	}
	if emptCount == 0 { // have solution
		solchan <- board
		return
	}
	for i := 0; i < bestOpt.n; i++ {
		if !modified || /*still*/ allowed(&board, bestOpt.cell, bestOpt.v[i]) {
			board[bestOpt.cell] = bestOpt.v[i]
			wgsol.Add(1)
			if runtime.NumGoroutine() < runtime.NumCPU() + extragoro {
				go solve(board)
			} else {
				solve(board)
			}
		}
	}
}

type errT string

func (e errT) Error() string {
	return string(e)
}

func readBoard(rd io.Reader) (board boardT, err error) {
	var i int
	var n int
	var b [1]byte

	for i < boardSize {
		n, err = rd.Read(b[:1])
		if err != nil {
			return
		}
		if n != 1 {
			err = errT("unexpected end of input")
			return
		}
		c := b[0]
		if c >= '1' && c <= '9' {
			board[i] = digitT(c - '0')
			i++
			continue
		}
		if c > 0x20 {
			board[i] = 0
			i++
			continue
		}
	}
	return
}

type wbufferT struct {
	buff []byte
	n    int
}

func (wb *wbufferT) put(b byte) {
	wb.buff[wb.n] = b
	wb.n++
}

func writeBoard(wr io.Writer, board boardT) (n int, err error) {
	var buff [4 * boardSize]byte
	wb := wbufferT{buff[:], 0}

	for i := 0; i < boardSize; i++ {
		if i%rootNum == 0 {
			wb.put(0x20)
		}
		wb.put(0x20)
		if board[i] == 0 {
			wb.put('.')
		} else {
			wb.put('0' + byte(board[i]))
		}
		if (i+1)%sideLen == 0 {
			wb.put('\n')
			if (i+1)%(rootNum*sideLen) == 0 && i != boardSize-1 {
				wb.put('\n')
			}
		}
	}
	n, err = wr.Write(wb.buff[:wb.n])
	return
}

func outputter(wr io.Writer) {
	defer wgout.Done()
	var n int
	for board := range solchan {
		n += 1
		wr.Write([]byte(fmt.Sprintf("Solution %d:\n", n)))
		writeBoard(wr, board)
	}
	wr.Write([]byte(fmt.Sprintf("Number of solutions: %d\n", n)))
}

func main() {
	if len(os.Args) > 2 {
		fmt.Sscan(os.Args[2], &extragoro)
	} else {
		extragoro = 1 // count in outputter which is mostly idle
	}
	f, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Println("%v", err.Error())
		return
	}
	board, err := readBoard(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", err.Error())
		return
	}
	f.Close()
	writeBoard(os.Stdout, board)

	solchan = make(chan boardT)
	wgout.Add(1)
	go outputter(os.Stdout)

	t1 := time.Now()
	make_groups()
	wgsol.Add(1)
	solve(board)
	wgsol.Wait()
	t2 := time.Now()
	close(solchan)
	wgout.Wait()
	fmt.Printf("Time elapsed: %v\n", t2.Sub(t1))
}
