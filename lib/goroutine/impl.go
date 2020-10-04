package goroutine

func newGoroutine() *Goroutine {
	channel := make(chan Function)
	go managerLoop(channel)
	return &Goroutine{channel}
}

func managerLoop(start <-chan Function) {
	for {
		fn, ok := <-start
		if !ok {
			return
		}
		if fn != nil {
			fn()
		}
	}
}
