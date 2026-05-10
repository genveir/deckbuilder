package combat

import (
	"container/heap"
	"math"
)

// Grid + A* navigation overlay.
//
// The world is tiled into a fixed-extent square grid of cellSize-unit cells.
// Walls rasterize onto cells whose center is within ~0.6*cellSize of the
// segment, so any cell along (or hugging) a wall is impassable. Past the
// endpoints, cells are open — enemies can route around walls by going
// around the ends.

const (
	cellSize = 12.0
	gridHalf = 60 // cells per axis from origin; world covers ~[-720, 720]
	gridDim  = gridHalf * 2
)

type cellPos struct{ X, Y int }

type navGrid struct {
	blocked [gridDim * gridDim]bool
	dirty   bool
}

func cellIdx(cx, cy int) int { return cy*gridDim + cx }

func cellInBounds(cx, cy int) bool {
	return cx >= 0 && cx < gridDim && cy >= 0 && cy < gridDim
}

func worldToCell(wx, wy float64) (int, int) {
	cx := int(math.Floor(wx/cellSize)) + gridHalf
	cy := int(math.Floor(wy/cellSize)) + gridHalf
	return cx, cy
}

func cellToWorld(cx, cy int) (float64, float64) {
	return (float64(cx-gridHalf) + 0.5) * cellSize,
		(float64(cy-gridHalf) + 0.5) * cellSize
}

func (c *Combat) markGridDirty() {
	if c.grid != nil {
		c.grid.dirty = true
	}
}

func (c *Combat) ensureGrid() {
	if c.grid == nil {
		c.grid = &navGrid{dirty: true}
	}
	if !c.grid.dirty {
		return
	}
	for i := range c.grid.blocked {
		c.grid.blocked[i] = false
	}
	for _, w := range c.Walls {
		if w.HP <= 0 {
			continue
		}
		c.rasterizeWall(w)
	}
	c.grid.dirty = false
}

func (c *Combat) rasterizeWall(w *Wall) {
	minX := math.Min(w.X1, w.X2) - cellSize
	maxX := math.Max(w.X1, w.X2) + cellSize
	minY := math.Min(w.Y1, w.Y2) - cellSize
	maxY := math.Max(w.Y1, w.Y2) + cellSize
	cx1, cy1 := worldToCell(minX, minY)
	cx2, cy2 := worldToCell(maxX, maxY)
	if cx1 > cx2 {
		cx1, cx2 = cx2, cx1
	}
	if cy1 > cy2 {
		cy1, cy2 = cy2, cy1
	}
	const halfBand = cellSize * 0.6
	for cy := cy1; cy <= cy2; cy++ {
		for cx := cx1; cx <= cx2; cx++ {
			if !cellInBounds(cx, cy) {
				continue
			}
			wx, wy := cellToWorld(cx, cy)
			cpx, cpy := closestPointOnSegment(wx, wy, w.X1, w.Y1, w.X2, w.Y2)
			if math.Hypot(wx-cpx, wy-cpy) <= halfBand {
				c.grid.blocked[cellIdx(cx, cy)] = true
			}
		}
	}
}

func (c *Combat) cellBlocked(cx, cy int) bool {
	if !cellInBounds(cx, cy) {
		return true
	}
	return c.grid.blocked[cellIdx(cx, cy)]
}

// findNearestOpen searches outward in a square spiral for the nearest open
// cell, returning (-1,-1) if none found within the search radius.
func (c *Combat) findNearestOpen(cx, cy int) (int, int) {
	if !c.cellBlocked(cx, cy) {
		return cx, cy
	}
	const maxRadius = 8
	for r := 1; r <= maxRadius; r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if dx > -r && dx < r && dy > -r && dy < r {
					continue // interior of previous ring
				}
				nx := cx + dx
				ny := cy + dy
				if !c.cellBlocked(nx, ny) {
					return nx, ny
				}
			}
		}
	}
	return -1, -1
}

// --- A* ---

type pqEntry struct {
	pos cellPos
	f   float64
	idx int // heap index helper for stable ordering
}

type pqHeap []pqEntry

func (h pqHeap) Len() int            { return len(h) }
func (h pqHeap) Less(i, j int) bool  { return h[i].f < h[j].f }
func (h pqHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *pqHeap) Push(x interface{}) { *h = append(*h, x.(pqEntry)) }
func (h *pqHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

var dirs8 = [8]struct {
	dx, dy int
	cost   float64
}{
	{1, 0, 1}, {-1, 0, 1}, {0, 1, 1}, {0, -1, 1},
	{1, 1, math.Sqrt2}, {1, -1, math.Sqrt2},
	{-1, 1, math.Sqrt2}, {-1, -1, math.Sqrt2},
}

func octile(ax, ay, bx, by int) float64 {
	dx := math.Abs(float64(ax - bx))
	dy := math.Abs(float64(ay - by))
	return math.Max(dx, dy) + (math.Sqrt2-1)*math.Min(dx, dy)
}

// findPath returns the cell path from (sx,sy) to (gx,gy) inclusive, or nil
// if unreachable. Diagonal moves cannot squeeze through corners blocked on
// both orthogonal neighbours.
func (c *Combat) findPath(srcWX, srcWY, dstWX, dstWY float64) []cellPos {
	c.ensureGrid()
	sx, sy := worldToCell(srcWX, srcWY)
	gx, gy := worldToCell(dstWX, dstWY)
	if !cellInBounds(sx, sy) || !cellInBounds(gx, gy) {
		return nil
	}
	if c.cellBlocked(sx, sy) {
		nsx, nsy := c.findNearestOpen(sx, sy)
		if nsx < 0 {
			return nil
		}
		sx, sy = nsx, nsy
	}
	if c.cellBlocked(gx, gy) {
		ngx, ngy := c.findNearestOpen(gx, gy)
		if ngx < 0 {
			return nil
		}
		gx, gy = ngx, ngy
	}
	if sx == gx && sy == gy {
		return []cellPos{{sx, sy}}
	}

	nodes := make(map[cellPos]*pathNode, 64)
	open := &pqHeap{}
	heap.Init(open)

	start := cellPos{sx, sy}
	goal := cellPos{gx, gy}
	nodes[start] = &pathNode{g: 0}
	heap.Push(open, pqEntry{pos: start, f: octile(sx, sy, gx, gy)})

	const expansionCap = 4000
	expansions := 0
	for open.Len() > 0 {
		e := heap.Pop(open).(pqEntry)
		expansions++
		if expansions > expansionCap {
			return nil
		}
		cur := e.pos
		ni := nodes[cur]
		if ni.closed {
			continue
		}
		ni.closed = true
		if cur == goal {
			return reconstruct(nodes, cur)
		}
		for _, d := range dirs8 {
			nx := cur.X + d.dx
			ny := cur.Y + d.dy
			if !cellInBounds(nx, ny) || c.cellBlocked(nx, ny) {
				continue
			}
			// disallow corner-cutting through a diagonal pair where both
			// orthogonal neighbours are blocked
			if d.dx != 0 && d.dy != 0 {
				if c.cellBlocked(cur.X+d.dx, cur.Y) || c.cellBlocked(cur.X, cur.Y+d.dy) {
					continue
				}
			}
			np := cellPos{nx, ny}
			tentative := ni.g + d.cost
			ex, exists := nodes[np]
			if !exists {
				ex = &pathNode{g: math.Inf(1)}
				nodes[np] = ex
			}
			if tentative >= ex.g {
				continue
			}
			ex.parent = cur
			ex.hasP = true
			ex.g = tentative
			f := tentative + octile(nx, ny, gx, gy)
			heap.Push(open, pqEntry{pos: np, f: f})
		}
	}
	return nil
}

func reconstruct(nodes map[cellPos]*pathNode, end cellPos) []cellPos {
	path := []cellPos{end}
	cur := end
	for {
		n := nodes[cur]
		if n == nil || !n.hasP {
			break
		}
		cur = n.parent
		path = append(path, cur)
	}
	// reverse
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}

type pathNode struct {
	parent cellPos
	g      float64
	closed bool
	hasP   bool
}
