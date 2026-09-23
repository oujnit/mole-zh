package plugin

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed rules.json
var rulesData []byte

type Rule struct {
	File   string `json:"file"`
	Before string `json:"before"`
	After  string `json:"after"`
}

type Catalog struct {
	Base  string `json:"base"`
	Rules []Rule `json:"rules"`
}

func loadCatalog() (Catalog, error) {
	var c Catalog
	if err := json.Unmarshal(rulesData, &c); err != nil {
		return c, err
	}
	for _, r := range c.Rules {
		if r.File == "" || strings.HasPrefix(r.File, "/") || strings.Contains(r.File, "..") ||
			!strings.HasPrefix(r.File, "bin/") && !strings.HasPrefix(r.File, "lib/") && !strings.HasPrefix(r.File, "cmd/") && r.File != "mole" {
			return c, fmt.Errorf("unsafe catalog path %q", r.File)
		}
		if r.Before == "" || r.Before == r.After || strings.ContainsAny(r.Before, "\r\n") || strings.ContainsAny(r.After, "\r\n") {
			return c, fmt.Errorf("invalid catalog rule in %s", r.File)
		}
	}
	return c, nil
}

func translateLines(data []byte, rules []Rule) ([]byte, int, []string) {
	lines := strings.Split(string(data), "\n")
	replacements := make(map[string]string, len(rules))
	counts := make(map[string]int, len(rules))
	for _, rule := range rules {
		replacements[rule.Before] = rule.After
	}
	applied := 0
	var skipped []string
	for i, line := range lines {
		if after, ok := replacements[line]; ok {
			lines[i] = after
			counts[line]++
			applied++
		}
	}
	for _, rule := range rules {
		if counts[rule.Before] == 0 {
			skipped = append(skipped, rule.File+": "+strings.TrimSpace(rule.Before))
		}
	}
	return []byte(strings.Join(lines, "\n")), applied, skipped
}
