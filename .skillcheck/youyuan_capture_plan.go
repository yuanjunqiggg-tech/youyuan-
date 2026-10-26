package main

// Planning is pure and testable. No camera request, login or teleport is issued
// here. Unknown dimensions stay unknown; an OP role is not evidence of coverage.
import (
	"fmt"
	"sort"
)

// Fleet steps are deployment targets, not evidence that an account has joined.
// An empty policy preserves existing installations' deployment behavior.
type yyCaptureFleetStep struct {
	MinPlayers int `json:"min_players"`
	Observers  int `json:"observers"`
}

func yyCaptureFleetStepsValidate(steps []yyCaptureFleetStep) error {
	if len(steps) > yyAccountProcessSlots {
		return fmt.Errorf("人数阶梯最多%d行", yyAccountProcessSlots)
	}
	previous := 0
	for _, s := range steps {
		if s.MinPlayers <= previous || s.MinPlayers > 1000 || s.Observers < 1 || s.Observers > yyAccountProcessSlots {
			return fmt.Errorf("人数阶梯需按真人数递增且不重复，真人数1..1000，机器人数量1..%d", yyAccountProcessSlots)
		}
		previous = s.MinPlayers
	}
	if len(steps) > 0 && steps[0].MinPlayers != 1 {
		return fmt.Errorf("人数阶梯第一行必须从1名真人开始")
	}
	return nil
}

func yyCaptureResidentCount(c yyGlobalCaptureConfig) int {
	if c.DimensionRouting != nil && !*c.DimensionRouting {
		return 0
	}
	if c.ResidentObservers == 0 {
		return 3
	}
	return c.ResidentObservers
}

func yyCaptureConfiguredDesired(c yyGlobalCaptureConfig, players int) int {
	count := yyCaptureDesired(players, c.AccountLimit)
	if len(c.FleetSteps) != 0 {
		count = 0
		for _, step := range c.FleetSteps {
			if step.MinPlayers > players {
				break
			}
			count = step.Observers
		}
	}
	count = max(count, yyCaptureResidentCount(c))
	if c.AccountLimit > 0 {
		count = min(count, c.AccountLimit)
	}
	return min(count, yyAccountProcessSlots)
}

type yyCapturePlayer struct {
	Name      string `json:"name"`
	Dimension int32  `json:"dimension"`
	Known     bool   `json:"dimension_known"`
}
type yyCaptureAssignment struct {
	Account   string   `json:"account"`
	Dimension int32    `json:"dimension"`
	Players   []string `json:"players"`
	Phase     float64  `json:"phase_seconds"`
	Offset    int      `json:"target_offset"`
	Home      *int32   `json:"home_dimension,omitempty"`
	Borrowed  bool     `json:"borrowed"`
}

func yyCaptureDesired(n, limit int) int {
	count := n
	if count > 3 {
		count = 3
	}
	if n > 3 && limit > 0 {
		count = min(n, limit)
	}
	if n > 10 {
		count += 2
	}
	if n >= 15 {
		count++
	}
	if limit > 0 && count > limit {
		count = limit
	}
	return count
}
func yyCapturePlan(c yyGlobalCaptureConfig, players []yyCapturePlayer, ready []string) []yyCaptureAssignment {
	n := yyCaptureConfiguredDesired(c, len(players))
	if n > len(ready) {
		n = len(ready)
	}
	if n == 0 {
		return []yyCaptureAssignment{}
	}
	homes := yyGlobalDimensionAssignments(c)
	groups := map[int32][]string{}
	unknown := []string{}
	for _, p := range players {
		if p.Known {
			groups[p.Dimension] = append(groups[p.Dimension], p.Name)
		} else {
			unknown = append(unknown, p.Name)
		}
	}
	dims := []int32{}
	for d, v := range groups {
		if len(v) > 0 {
			sort.Strings(v)
			dims = append(dims, d)
		}
	}
	sort.Slice(dims, func(i, j int) bool { return dims[i] < dims[j] })
	if len(dims) == 0 {
		dims = []int32{0}
	} // Explicit probing only; never marks unknown players as dimension 0.
	chosen := []string{}
	used := map[string]bool{}
	counts := map[int32]int{}
	assigned := map[string]int32{}
	// Keep a resident in each configured dimension, even if it has no humans.
	// If credentials are insufficient, cover occupied dimensions first and
	// report the remaining shortfall rather than claiming three residents.
	residentDims := append([]int32{}, dims...)
	for _, d := range []int32{0, 1, 2} {
		found := false
		for _, x := range residentDims {
			if x == d {
				found = true
			}
		}
		if !found {
			residentDims = append(residentDims, d)
		}
	}
	for _, d := range residentDims[:min(len(residentDims), min(n, yyCaptureResidentCount(c)))] {
		id := ""
		for _, candidate := range ready {
			if h, ok := homes[candidate]; ok && h == d && !used[candidate] {
				id = candidate
				break
			}
		}
		if id == "" {
			for _, candidate := range ready {
				if !used[candidate] {
					id = candidate
					break
				}
			}
		}
		if id != "" {
			chosen = append(chosen, id)
			used[id] = true
			assigned[id] = d
			counts[d]++
		}
	}
	// Only additional observers lend capacity to the busiest dimension.
	for _, id := range ready {
		if used[id] || len(chosen) >= n {
			continue
		}
		best := dims[0]
		for _, d := range dims {
			if counts[d] == 0 {
				best = d
				break
			}
			if float64(len(groups[d]))/float64(counts[d]+1) > float64(len(groups[best]))/float64(counts[best]+1) {
				best = d
			}
		}
		chosen = append(chosen, id)
		used[id] = true
		assigned[id] = best
		counts[best]++
	}
	indices := map[int32]int{}
	out := []yyCaptureAssignment{}
	for _, id := range chosen {
		d := assigned[id]
		idx := indices[d]
		indices[d]++
		all := append(append([]string{}, groups[d]...), unknown...)
		targets := []string{}
		// Disjoint primary routes avoid all bots staring at one player. Helpers
		// with no own partition overlap and receive a different route offset/phase.
		for i, name := range all {
			if i%counts[d] == idx {
				targets = append(targets, name)
			}
		}
		if len(targets) == 0 {
			targets = all
		}
		a := yyCaptureAssignment{Account: id, Dimension: d, Players: targets, Phase: c.Interval * float64(idx) / float64(counts[d]), Offset: idx, Home: nil}
		if h, ok := homes[id]; ok {
			hcopy := h
			a.Home = &hcopy
			a.Borrowed = h != d
		}
		out = append(out, a)
	}
	return out
}
