package pool

import (
	"github.com/eapache/queue"
	"runtime"
	"sync"
)


type Queue struct {
	sync.Mutex
	popable *sync.Cond
	buffer  *queue.Queue
	closed  bool
}

func NewQueue() *Queue {
	e := &Queue{
		buffer: queue.New(),
	}
	e.popable = sync.NewCond(&e.Mutex)
	return e
}

func (e *Queue) Push(v interface{}) {
	e.Mutex.Lock()
	defer e.Mutex.Unlock()
	if !e.closed {
		e.buffer.Add(v)
		e.popable.Signal()
	}
}
func (e *Queue) Close() {
	e.Mutex.Lock()
	defer e.Mutex.Unlock()
	if !e.closed {
		e.closed = true
		e.popable.Broadcast() //广播
	}
}

//Pop 取出队列,（阻塞模式）
func (e *Queue) Pop() (v interface{}) {
	c := e.popable
	buffer := e.buffer

	e.Mutex.Lock()
	defer e.Mutex.Unlock()

	for buffer.Length() == 0 && !e.closed {
		c.Wait()
	}

	if e.closed { //已关闭
		return
	}

	if buffer.Length() > 0 {
		v = buffer.Peek()
		buffer.Remove()
	}
	return
}

//试着取出队列（非阻塞模式）返回ok == false 表示空
func (e *Queue) TryPop() (v interface{}, ok bool) {
	buffer := e.buffer

	e.Mutex.Lock()
	defer e.Mutex.Unlock()

	if buffer.Length() > 0 {
		v = buffer.Peek()
		buffer.Remove()
		ok = true
	} else if e.closed {
		ok = true
	}

	return
}

// Len returns the queue length. Safe for concurrent use.
func (e *Queue) Len() int {
	e.Mutex.Lock()
	defer e.Mutex.Unlock()
	return e.buffer.Length()
}

// Wait blocks until the queue is empty or closed.
func (e *Queue) Wait() {
	for {
		e.Mutex.Lock()
		done := e.closed || e.buffer.Length() == 0
		e.Mutex.Unlock()
		if done {
			break
		}
		runtime.Gosched()
	}
}
