package syncq

import (
	"container/list"
	"context"
)

// SyncQueue 类似于可无限buffer的channel
// 设置无限buffer的channel(max<=0)
// Enqueue 接口会阻塞直到可以元素放入队列中，阻塞的情况只在队列满的时候才会出现
// Dequeue 接口会阻塞直到队列中有元素返回，阻塞的情况只在队列空的时候才会出现
type SyncQueue struct {
	ctx    context.Context
	cancel context.CancelFunc

	l   *list.List
	max int
	in  chan interface{} // use to enqueue
	out chan interface{} // use to dequeue
}

// max代表队列元素个数上限，若小于等于0，则队列无元素上限
// 内部会启动一个goroutine用于channel同步，可用Destroy()方法销毁。
// 注意调用Destroy()后就不可执行入队出队操作，否则会一直阻塞下去。
func NewSyncQueueWithSize(max int) *SyncQueue {
	ctx, cancel := context.WithCancel(context.Background())
	q := &SyncQueue{
		ctx:    ctx,
		cancel: cancel,
		l:      list.New(),
		max:    max,
		in:     make(chan interface{}),
		out:    make(chan interface{}),
	}
	go q.dispatch()
	return q
}

func NewSyncQueue() *SyncQueue {
	return NewSyncQueueWithSize(0)
}

func (q *SyncQueue) dispatch() {
	for {
		if q.l.Len() == 0 {
			// the queue is empty, only enqueue is allowed.
			select {
			case v := <-q.in:
				q.l.PushBack(v)
			case <-q.ctx.Done():
				return
			}
		}
		e := q.l.Front()
		if q.max > 0 && q.l.Len() >= q.max {
			// the queue is full, only dequeue is allowed.
			select {
			case q.out <- e.Value:
				q.l.Remove(e)
			case <-q.ctx.Done():
				return
			}
		} else {
			// enqueue and dequeue are allowed.
			select {
			case value := <-q.in:
				q.l.PushBack(value)
			case q.out <- e.Value:
				q.l.Remove(e)
			case <-q.ctx.Done():
				return
			}
		}
	}
}

func (q *SyncQueue) Enqueue(value interface{}) {
	q.in <- value
}

func (q *SyncQueue) Dequeue() interface{} {
	return <-q.out
}

func (q *SyncQueue) EnqueueC() chan<- interface{} {
	return q.in
}

func (q *SyncQueue) DequeueC() <-chan interface{} {
	return q.out
}

func (q *SyncQueue) Destroy() {
	// cancel dispatch goroutine
	q.cancel()
}
