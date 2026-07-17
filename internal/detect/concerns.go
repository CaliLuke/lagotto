package detect

import "strings"

// detectConcerns returns the distinct concern groups present in a
// list of method names. Groups are identified by leading verb
// prefix; a method whose name starts with one of the known verbs
// belongs to that verb's group. A group needs at least three methods
// before it counts as evidence of an independent concern. Methods that
// don't match any verb fall into an "other" bucket with the same floor.
func detectConcerns(methods []string) []string {
	verbs := []struct {
		verbs  []string
		concer string
	}{
		// CRUD verbs describe one data-access responsibility on repositories,
		// managers, and query APIs. Treating every operation as an independent
		// concern makes complete persistence APIs look less cohesive than they
		// are. G1 should require responsibilities beyond CRUD breadth.
		{[]string{"Create", "Insert", "Add", "New", "Get", "List", "Find", "Read", "Load", "Fetch", "Update", "Edit", "Patch", "Set", "Replace", "Upsert", "Modify", "Delete", "Remove", "Drop", "Purge", "Clear", "Search", "Query", "Lookup"}, "data_access"},
		{[]string{"Merge", "Combine"}, "merge"},
		{[]string{"Run", "Execute", "Exec", "Apply", "Dispatch", "Schedule", "Process"}, "execute"},
		{[]string{"Check", "Validate", "Verify", "Audit"}, "validate"},
		{[]string{"Connect", "Disconnect", "Close", "Open", "Reconnect", "Reset"}, "connect"},
		{[]string{"Export", "Import", "Backup", "Restore", "Dump"}, "export"},
		{[]string{"Thread", "Bring", "Resolve"}, "thread"},
		{[]string{"Promote", "Approve", "Reject", "Propose"}, "promote"},
		{[]string{"Count", "Stats", "Status"}, "meta"},
		{[]string{"Format", "Render", "Encode", "Decode"}, "format"},
	}
	groupCounts := map[string]int{}
	other := 0
	for _, m := range methods {
		methodName := strings.ToLower(m)
		matched := false
		for _, v := range verbs {
			for _, prefix := range v.verbs {
				if strings.HasPrefix(methodName, strings.ToLower(prefix)) {
					groupCounts[v.concer]++
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			other++
		}
	}
	groupCounts["other"] = other
	groupSet := map[string]bool{}
	for concern, count := range groupCounts {
		if count >= 3 {
			groupSet[concern] = true
		}
	}
	return sortedKeys(groupSet)
}
