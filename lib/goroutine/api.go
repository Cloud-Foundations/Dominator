package goroutine

// Function defines a simple function with no arguments or return values. This
// is typically used as a closure.
type Function func()

// Goroutine represents a goroutine. A Goroutine is useful for running functions
// in a specific environment (such as a pinned OS thread with modified
// attributes).
type Goroutine struct {
	start chan<- Function
}

// New creates a Goroutine with an underlying goroutine which can run functions.
func New() *Goroutine {
	return newGoroutine()
}

// Quit causes the underlying goroutine to exit. If there is a currently running
// function, it will wait for it to complete.
func (g *Goroutine) Quit() {
	close(g.start)
}

// Run will run a function in the underlying goroutine and wait for it to
// complete. If there is a currently running function, it will first wait for it
// to complete.
func (g *Goroutine) Run(fn Function) {
	g.start <- fn
	g.start <- nil
}

// Start will run a function in the underlying goroutine. If there is a
// currently running function, it will first wait for it to complete.
func (g *Goroutine) Start(fn Function) {
	g.start <- fn
}

// Wait will wait for a running function to complete.
func (g *Goroutine) Wait() {
	g.start <- nil
}
