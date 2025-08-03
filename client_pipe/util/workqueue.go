package util

import (
	"sync"
	"time"
)

// WorkQueue 工作队列接口
type WorkQueue interface {
	Add(item interface{})
	Get() (item interface{}, shutdown bool)
	Done(item interface{})
	ShutDown()
	Len() int
}

// workQueue 工作队列实现
type workQueue struct {
	queue    []interface{}
	dirty    map[interface{}]struct{}
	processing map[interface{}]struct{}
	
	cond     *sync.Cond
	shutdown bool
}

// NewWorkQueue 创建工作队列
func NewWorkQueue() WorkQueue {
	return &workQueue{
		queue:      make([]interface{}, 0),
		dirty:      make(map[interface{}]struct{}),
		processing: make(map[interface{}]struct{}),
		cond:       sync.NewCond(&sync.Mutex{}),
	}
}

// Add 添加项目
func (q *workQueue) Add(item interface{}) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	
	if q.shutdown {
		return
	}
	
	if _, exists := q.dirty[item]; exists {
		return
	}
	
	q.dirty[item] = struct{}{}
	if _, exists := q.processing[item]; !exists {
		q.queue = append(q.queue, item)
	}
	
	q.cond.Signal()
}

// Get 获取项目
func (q *workQueue) Get() (item interface{}, shutdown bool) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	
	for len(q.queue) == 0 && !q.shutdown {
		q.cond.Wait()
	}
	
	if len(q.queue) == 0 {
		return nil, true
	}
	
	item = q.queue[0]
	q.queue = q.queue[1:]
	
	q.processing[item] = struct{}{}
	delete(q.dirty, item)
	
	return item, false
}

// Done 完成项目
func (q *workQueue) Done(item interface{}) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	
	delete(q.processing, item)
	
	if _, exists := q.dirty[item]; exists {
		q.queue = append(q.queue, item)
		q.cond.Signal()
	}
}

// ShutDown 关闭队列
func (q *workQueue) ShutDown() {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	
	q.shutdown = true
	q.cond.Broadcast()
}

// Len 队列长度
func (q *workQueue) Len() int {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	
	return len(q.queue)
}

// DelayingWorkQueue 延迟工作队列接口
type DelayingWorkQueue interface {
	WorkQueue
	AddAfter(item interface{}, duration time.Duration)
}

// delayingWorkQueue 延迟工作队列实现
type delayingWorkQueue struct {
	WorkQueue
	
	clock    Clock
	stopCh   chan struct{}
	stopOnce sync.Once
}

// Clock 时钟接口
type Clock interface {
	Now() time.Time
	After(duration time.Duration) <-chan time.Time
}

// realClock 真实时钟
type realClock struct{}

func (realClock) Now() time.Time                         { return time.Now() }
func (realClock) After(duration time.Duration) <-chan time.Time { return time.After(duration) }

// NewDelayingWorkQueue 创建延迟工作队列
func NewDelayingWorkQueue() DelayingWorkQueue {
	return &delayingWorkQueue{
		WorkQueue: NewWorkQueue(),
		clock:     &realClock{},
		stopCh:    make(chan struct{}),
	}
}

// AddAfter 延迟添加
func (q *delayingWorkQueue) AddAfter(item interface{}, duration time.Duration) {
	if duration <= 0 {
		q.Add(item)
		return
	}
	
	go func() {
		select {
		case <-q.clock.After(duration):
			q.Add(item)
		case <-q.stopCh:
			return
		}
	}()
}

// ShutDown 关闭队列
func (q *delayingWorkQueue) ShutDown() {
	q.WorkQueue.ShutDown()
	q.stopOnce.Do(func() {
		close(q.stopCh)
	})
}