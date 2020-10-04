package goroutine

type Function func()

type Goroutine struct {
	start chan<- Function
}

func New() *Goroutine {
	return newGoroutine()
}

func (g *Goroutine) Quit() {
	close(g.start)
}

func (g *Goroutine) Run(fn Function) {
	g.start <- fn
	g.start <- nil
}

func (g *Goroutine) Start(fn Function) {
	g.start <- fn
}

func (g *Goroutine) Wait() {
	g.start <- nil
}
