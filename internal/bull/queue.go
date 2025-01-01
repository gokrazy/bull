package bull

import "context"

// queue is an unbounded queue.
//
// see Bryan C. Mills: Rethinking Classical Concurrency Patterns (GopherCon 2018)
// https://drive.google.com/file/d/1nPdvhB0PutEJzdCq5ms6UI58dp50fcAN/view
type queue struct {
	items chan []string
	empty chan bool
}

func newQueue() *queue {
	items := make(chan []string, 1)
	empty := make(chan bool, 1)
	empty <- true
	return &queue{
		items: items,
		empty: empty,
	}
}

func (q *queue) Push(dir string) {
	var items []string
	select {
	case items = <-q.items:
	case <-q.empty:
	}
	items = append(items, dir)
	q.items <- items
}

func (q *queue) PopOrWait(ctx context.Context) (string, error) {
	var items []string
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case items = <-q.items:
	}
	item := items[0]
	if len(items) == 1 {
		q.empty <- true
	} else {
		q.items <- items[1:]
	}
	return item, nil
}
