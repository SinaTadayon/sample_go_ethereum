package bsc_node

import (
	log "github.com/sirupsen/logrus"
	"runtime"
	"strconv"
)

type Task func()

var workerCounter int64 = 0
// Worker worker is the actual executor who runs the tasks,
// it starts a goroutine that accepts tasks and
// performs function calls.
type Worker struct {
	// identity
	name string

	// PanicHandler is used to handle panics of worker goroutine.
	// if nil, panics will be thrown out again from worker goroutines.
	PanicHandler func(interface{})
}

func workerFactory() *Worker {
	workerCounter++
	return &Worker{
		name:         "worker_" + strconv.Itoa(int(workerCounter)),
		PanicHandler: nil,
	}
}

func workerWithPanicHandlerFactory(fn func(interface{})) *Worker {
	workerCounter++
	return &Worker{
		name:         "worker_" + strconv.Itoa(int(workerCounter)),
		PanicHandler: fn,
	}
}


// run starts a goroutine to repeat the process
// that performs the function calls.
func (w *Worker) run(task Task) {
	go func() {
		defer func() {
			if p := recover(); p != nil {
				if ph := w.PanicHandler; ph != nil {
					ph(p)
				} else {
					log.Error("worker exits from a panic, worker %s, panic: %s", w.name, p)
					var buf [4096]byte
					n := runtime.Stack(buf[:], false)
					log.Error("worker exits from panic, worker: %s, stack: %s", w.name, string(buf[:n]))
				}
			}
		}()

		log.Debug("%s Launch task", w.name)
		// launch task
		task()
	}()
}
