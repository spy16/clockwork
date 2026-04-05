package inmem

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/spy16/clockwork"
	"github.com/spy16/clockwork/schedule"
)

var _ schedule.Scheduler = (*Scheduler)(nil)

// Scheduler implements a clockwork scheduler using in-memory primitives. This is
// meant for easy end-to-end testing only and does not work in a distributed
// mode.
type Scheduler struct {
	mu        sync.RWMutex
	schedules map[string]schedule.Schedule
	requests  requestQ
}

func (sched *Scheduler) Get(ctx context.Context, scheduleID string) (*schedule.Schedule, error) {
	sched.mu.RLock()
	defer sched.mu.RUnlock()

	sc, found := sched.schedules[scheduleID]
	if !found {
		return nil, clockwork.ErrNotFound
	}
	return &sc, nil
}

func (sched *Scheduler) Put(_ context.Context, sc schedule.Schedule, isUpdate bool, requests ...schedule.Execution) error {
	sched.mu.Lock()
	defer sched.mu.Unlock()

	_, exists := sched.schedules[sc.ID]
	if exists && !isUpdate {
		return clockwork.ErrConflict
	} else if !exists && isUpdate {
		return clockwork.ErrNotFound
	}

	if sched.schedules == nil {
		sched.schedules = map[string]schedule.Schedule{}
	}
	sched.schedules[sc.ID] = sc
	sched.requests.Enqueue(requests...)
	return nil
}

func (sched *Scheduler) Del(_ context.Context, scheduleID string) error {
	sched.mu.Lock()
	defer sched.mu.Unlock()

	_, found := sched.schedules[scheduleID]
	if !found {
		return clockwork.ErrNotFound
	}
	delete(sched.schedules, scheduleID)
	return nil
}

func (sched *Scheduler) List(_ context.Context, offset, count int) ([]schedule.Schedule, error) {
	sched.mu.RLock()
	defer sched.mu.RUnlock()

	i := 0
	schedules := make([]schedule.Schedule, len(sched.schedules))
	for _, sc := range sched.schedules {
		schedules[i] = sc
		i++
	}

	// sort by createdAt desc
	sort.Slice(schedules, func(i, j int) bool {
		return schedules[i].CreatedAt.Before(schedules[j].CreatedAt)
	})

	if count == -1 || offset+count >= len(schedules) {
		count = len(schedules) - offset
	}

	return schedules[offset : offset+count], nil
}

func (sched *Scheduler) Run(ctx context.Context, onReady schedule.OnReadyFunc) error {
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-tick.C:
			t := time.Now()
			for {
				readyReq := sched.requests.Dequeue(t)
				if readyReq == nil {
					break
				}
				sched.handle(ctx, onReady, *readyReq)
			}
		}
	}
}

func (sched *Scheduler) handle(ctx context.Context, onReady schedule.OnReadyFunc, curReq schedule.Execution) {
	sc, err := sched.Get(ctx, curReq.ScheduleID)
	if err != nil {
		if errors.Is(err, clockwork.ErrNotFound) {
			return
		}
		return
	}

	if sc.Version > curReq.Version {
		return
	}

	nextReq, err := onReady(ctx, *sc, curReq)
	if err != nil {
		sched.requests.Enqueue(curReq)
	} else if nextReq == nil {
		if sc.Status != schedule.StatusDisabled {
			sc.Status = schedule.StatusDone
		}
	} else {
		sched.requests.Enqueue(*nextReq)
		sc.NextExecutionAt = nextReq.EnqueueAt
		sc.EnqueueCount++
	}

	sched.mu.Lock()
	defer sched.mu.Unlock()
	sched.schedules[sc.ID] = *sc
}

func (sched *Scheduler) Stats(_ context.Context) (map[string]any, error) {
	return map[string]any{
		"schedules": len(sched.schedules),
		"enqueued":  sched.requests.Size(),
	}, nil
}

type requestQ struct {
	mu    sync.Mutex
	items []schedule.Execution
}

// Enqueue adds the given item to the queue.
func (q *requestQ) Enqueue(reqs ...schedule.Execution) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for _, req := range reqs {
		q.items = append(q.items, req)
		q.heapifyUp(q.Size() - 1)
	}
}

// Dequeue pops the next item from the queue and returns it. If the
// queue is empty, nil is returned.
func (q *requestQ) Dequeue(t time.Time) *schedule.Execution {
	if q.Size() == 0 {
		return nil
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	item := q.items[0]
	if item.EnqueueAt.After(t) {
		return nil
	}

	q.swap(0, q.Size()-1)          // swap with last element
	q.items = q.items[:q.Size()-1] // remove last element
	q.heapifyDown(0)               // bubble down

	return &item
}

// Size returns the number of items in the queue.
func (q *requestQ) Size() int { return len(q.items) }

func (q *requestQ) heapifyUp(index int) {
	parentAt := parent(index)
	if index > 0 {
		child := q.items[index]
		parent := q.items[parent(index)]
		if child.EnqueueAt.Before(parent.EnqueueAt) {
			q.swap(index, parentAt)
		}

		q.heapifyUp(parentAt)
	}
}

func (q *requestQ) heapifyDown(index int) {
	rightChildAt := rightChild(index)
	leftChildAt := leftChild(index)

	if index < q.Size() && leftChildAt < q.Size() && rightChildAt < q.Size() {
		parent := q.items[index]
		if parent.EnqueueAt.After(q.items[rightChildAt].EnqueueAt) {
			q.swap(rightChildAt, index)
			q.heapifyDown(rightChildAt)
		} else if parent.EnqueueAt.After(q.items[leftChildAt].EnqueueAt) {
			q.swap(leftChildAt, index)
			q.heapifyDown(leftChildAt)
		}
	}
}

func (q *requestQ) swap(i, j int) {
	tmp := q.items[i]
	q.items[i] = q.items[j]
	q.items[j] = tmp
}

func parent(index int) int {
	if index == 0 {
		return 0
	}

	return (index - 1) / 2
}

func leftChild(index int) int  { return 2*index + 1 }
func rightChild(index int) int { return 2*index + 2 }
