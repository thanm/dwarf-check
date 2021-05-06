package main

//go:noinline
func ABC(p1, p2 int, f1, f2 float64) {
	println(p1, p2, f1, f2)
}

func main() {
	ABC(1, 3, 5.0, 7.0)
}
