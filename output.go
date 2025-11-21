package main

type Point struct {
	x	int
	y	int
}
type Shape interface {
	isShape()
}
type Circle struct {
	Field0 int
}

func (v Circle) isShape() {
}

type Rect struct {
	Field0	int
	Field1	int
}

func (v Rect) isShape() {
}

const PI int = 3

func consume(x int) {
}
func consumePoint(p Point) {
}
func main() {
	x := PI
	consume(x)
	p := Point{x: 10, y: 20}
	consumePoint(p)
}
