package call

import "sort"

// Guess returns up to three of candidates most similar to word, with a
// similarity of at least 0.8: TTP's utils.guess, which is Python's
// difflib.get_close_matches(word, candidates, n=3, cutoff=0.8).
func Guess(word string, candidates []string) []string {
	return closeMatches(word, candidates, 3, 0.8)
}

// closeMatches ports difflib.get_close_matches. Strings are compared as
// sequences of runes, as Python compares str characters.
func closeMatches(word string, possibilities []string, n int, cutoff float64) []string {
	type scored struct {
		score float64
		s     string
	}
	b := []rune(word)
	m := newMatcher(b)
	var result []scored
	for _, x := range possibilities {
		a := []rune(x)
		if m.realQuickRatio(a) >= cutoff && m.quickRatio(a) >= cutoff {
			if r := m.ratio(a); r >= cutoff {
				result = append(result, scored{r, x})
			}
		}
	}
	// heapq.nlargest(n, result) on (score, x) tuples: the highest score
	// first, and of equal scores the greater string first.
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].score != result[j].score {
			return result[i].score > result[j].score
		}
		return result[i].s > result[j].s
	})
	if len(result) > n {
		result = result[:n]
	}
	out := make([]string, len(result))
	for i, r := range result {
		out[i] = r.s
	}
	return out
}

// matcher is difflib.SequenceMatcher with seq2 fixed (b) and seq1 set per
// comparison, no isjunk, and autojunk on.
type matcher struct {
	b       []rune
	b2j     map[rune][]int
	full    map[rune]int // fullbcount, for quick_ratio
	popular map[rune]bool
}

func newMatcher(b []rune) *matcher {
	m := &matcher{b: b, b2j: make(map[rune][]int)}
	for i, r := range b {
		m.b2j[r] = append(m.b2j[r], i)
	}
	// autojunk: with 200 or more items, an item making up more than 1% of
	// b (plus one) is "popular" and dropped from b2j.
	if n := len(b); n >= 200 {
		ntest := n/100 + 1
		for r, idx := range m.b2j {
			if len(idx) > ntest {
				if m.popular == nil {
					m.popular = make(map[rune]bool)
				}
				m.popular[r] = true
			}
		}
		for r := range m.popular {
			delete(m.b2j, r)
		}
	}
	m.full = make(map[rune]int)
	for _, r := range b {
		m.full[r]++
	}
	return m
}

func calcRatio(matches, length int) float64 {
	if length > 0 {
		return 2.0 * float64(matches) / float64(length)
	}
	return 1.0
}

func (m *matcher) realQuickRatio(a []rune) float64 {
	la, lb := len(a), len(m.b)
	return calcRatio(min(la, lb), la+lb)
}

func (m *matcher) quickRatio(a []rune) float64 {
	avail := make(map[rune]int)
	matches := 0
	for _, r := range a {
		n, ok := avail[r]
		if !ok {
			n = m.full[r]
		}
		avail[r] = n - 1
		if n > 0 {
			matches++
		}
	}
	return calcRatio(matches, len(a)+len(m.b))
}

func (m *matcher) ratio(a []rune) float64 {
	matches := 0
	for _, blk := range m.matchingBlocks(a) {
		matches += blk[2]
	}
	return calcRatio(matches, len(a)+len(m.b))
}

// findLongestMatch is SequenceMatcher.find_longest_match without junk
// (isjunk is None, so only popular items count as junk).
func (m *matcher) findLongestMatch(a []rune, alo, ahi, blo, bhi int) (int, int, int) {
	b := m.b
	besti, bestj, bestsize := alo, blo, 0
	j2len := map[int]int{}
	for i := alo; i < ahi; i++ {
		newj2len := map[int]int{}
		for _, j := range m.b2j[a[i]] {
			if j < blo {
				continue
			}
			if j >= bhi {
				break
			}
			k := j2len[j-1] + 1
			newj2len[j] = k
			if k > bestsize {
				besti, bestj, bestsize = i-k+1, j-k+1, k
			}
		}
		j2len = newj2len
	}
	// Extend the match with popular (junk) items on both sides.
	for besti > alo && bestj > blo && m.popular[b[bestj-1]] && a[besti-1] == b[bestj-1] {
		besti, bestj, bestsize = besti-1, bestj-1, bestsize+1
	}
	for besti+bestsize < ahi && bestj+bestsize < bhi && m.popular[b[bestj+bestsize]] && a[besti+bestsize] == b[bestj+bestsize] {
		bestsize++
	}
	return besti, bestj, bestsize
}

// matchingBlocks is SequenceMatcher.get_matching_blocks without the
// collapsing of adjacent blocks, which doesn't change their total size.
func (m *matcher) matchingBlocks(a []rune) [][3]int {
	la, lb := len(a), len(m.b)
	queue := [][4]int{{0, la, 0, lb}}
	var blocks [][3]int
	for len(queue) > 0 {
		q := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		alo, ahi, blo, bhi := q[0], q[1], q[2], q[3]
		i, j, k := m.findLongestMatch(a, alo, ahi, blo, bhi)
		if k == 0 {
			continue
		}
		blocks = append(blocks, [3]int{i, j, k})
		if alo < i && blo < j {
			queue = append(queue, [4]int{alo, i, blo, j})
		}
		if i+k < ahi && j+k < bhi {
			queue = append(queue, [4]int{i + k, ahi, j + k, bhi})
		}
	}
	return blocks
}
