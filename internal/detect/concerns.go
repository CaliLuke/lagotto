package detect

import "strings"

// detectConcerns returns the distinct concern groups present in a
// list of method names. Groups are identified by leading verb
// prefix; a method whose name starts with one of the known verbs
// belongs to that verb's group. Methods that don't match any verb
// fall into an "other" bucket.
func detectConcerns(methods []string) []string {
	verbs := []struct {
		verbs  []string
		concer string
	}{
		{[]string{"Create", "Insert", "Add", "New"}, "create"},
		{[]string{"Get", "List", "Find", "Read", "Load", "Fetch"}, "read"},
		{[]string{"Update", "Edit", "Patch", "Set", "Replace", "Upsert", "Modify"}, "update"},
		{[]string{"Delete", "Remove", "Drop", "Purge", "Clear"}, "delete"},
		{[]string{"Merge", "Combine"}, "merge"},
		{[]string{"Search", "Query", "Lookup"}, "search"},
		{[]string{"Run", "Execute", "Exec", "Apply"}, "execute"},
		{[]string{"Check", "Validate", "Verify", "Audit"}, "validate"},
		{[]string{"Connect", "Disconnect", "Close", "Open", "Reconnect", "Reset"}, "connect"},
		{[]string{"Export", "Import", "Backup", "Restore", "Dump"}, "export"},
		{[]string{"Thread", "Bring", "Resolve"}, "thread"},
		{[]string{"Promote", "Approve", "Reject", "Propose"}, "promote"},
		{[]string{"Count", "Stats", "Status"}, "meta"},
		{[]string{"Format", "Render", "Encode", "Decode"}, "format"},
	}
	groupSet := map[string]bool{}
	other := 0
	for _, m := range methods {
		matched := false
		for _, v := range verbs {
			for _, prefix := range v.verbs {
				if strings.HasPrefix(m, prefix) {
					groupSet[v.concer] = true
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
	if other >= 3 {
		groupSet["other"] = true
	}
	return sortedKeys(groupSet)
}
