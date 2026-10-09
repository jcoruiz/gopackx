package placement

import "github.com/jcoruiz/gopackx/pkg/model"

// maxBinStates bounds how many bins an engine remembers, as a safety net
// for engines reused across many packing runs without Reset. A single run
// keeps far fewer bins open.
const maxBinStates = 4096

// binStates remembers an engine's per-bin state (extreme points, free
// spaces, levels) so that switching between open bins does not rebuild it
// from the bin's items every time. When full, it forgets the bin used
// least recently.
type binStates[T any] struct {
	m     map[*model.Bin]savedState[T]
	clock uint64
}

type savedState[T any] struct {
	state T
	n     int         // len(bin.Items) when saved
	last  *model.Item // last item in the bin when saved
	used  uint64      // clock value of the last save
}

// save stores the state of bin as it is now.
func (c *binStates[T]) save(bin *model.Bin, state T) {
	if c.m == nil {
		c.m = make(map[*model.Bin]savedState[T])
	}
	if _, ok := c.m[bin]; !ok && len(c.m) >= maxBinStates {
		c.evictOldest()
	}
	c.clock++
	s := savedState[T]{state: state, n: len(bin.Items), used: c.clock}
	if s.n > 0 {
		s.last = bin.Items[s.n-1]
	}
	c.m[bin] = s
}

func (c *binStates[T]) evictOldest() {
	var oldest *model.Bin
	var at uint64
	for b, s := range c.m {
		if oldest == nil || s.used < at {
			oldest, at = b, s.used
		}
	}
	delete(c.m, oldest)
}

// reset forgets every bin.
func (c *binStates[T]) reset() {
	c.m = nil
}

// load returns the saved state of bin if the bin has not changed since it
// was saved by this engine.
func (c *binStates[T]) load(bin *model.Bin) (T, bool) {
	s, ok := c.m[bin]
	if !ok || s.n != len(bin.Items) || (s.n > 0 && bin.Items[s.n-1] != s.last) {
		var zero T
		return zero, false
	}
	return s.state, true
}
