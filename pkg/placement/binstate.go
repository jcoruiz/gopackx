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
	rev   uint64 // bin revision the state matches
	used  uint64 // clock value of the last save
}

// save stores state, which matches revision rev of bin.
func (c *binStates[T]) save(bin *model.Bin, state T, rev uint64) {
	if c.m == nil {
		c.m = make(map[*model.Bin]savedState[T])
	}
	if _, ok := c.m[bin]; !ok && len(c.m) >= maxBinStates {
		c.evictOldest()
	}
	c.clock++
	c.m[bin] = savedState[T]{state: state, rev: rev, used: c.clock}
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

// load returns the saved state of bin if it matches the bin's current
// revision, that is, if nothing was placed in or removed from the bin since.
func (c *binStates[T]) load(bin *model.Bin) (T, bool) {
	s, ok := c.m[bin]
	if !ok || s.rev != bin.Revision() {
		var zero T
		return zero, false
	}
	return s.state, true
}
