package loadconfig

import (
	"regexp"
	"strings"

	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"
)

func (c *checker) warnings(cfg model.LoadConfig, others []model.LoadConfig) {
	c.warnPattern("patterns.preprocess", cfg.Patterns.Preprocess)
	c.warnPattern("patterns.ingest", cfg.Patterns.Ingest)
	c.warnMode(cfg)
	c.warnPlaceholders(cfg)
	c.warnDelimiter(cfg)
	c.warnColumnNames(cfg)
	c.warnClassificationMismatch(cfg)
	c.warnOverlaps(cfg, others)
}

func (c *checker) warnPattern(field, pattern string) {
	if pattern == "" {
		return
	}

	if _, err := compilePattern(pattern); err != nil {
		return
	}

	if unanchored(pattern) {
		c.warn(field, "regex is unanchored; anchor it with ^ and $ so it cannot match a longer path")
	}

	if hasUnescapedDot(pattern) {
		c.warn(field, "regex has an unescaped dot; escape filename dots such as tar\\.gz")
	}
}

func unanchored(pattern string) bool {
	return !strings.HasPrefix(pattern, "^") || !strings.HasSuffix(pattern, "$")
}

func hasUnescapedDot(pattern string) bool {
	inClass := false

	for i := 0; i < len(pattern); i++ {
		char := pattern[i]
		if char == '\\' && !inClass {
			i++

			continue
		}

		if char == '[' && !inClass {
			inClass = true

			continue
		}

		if char == ']' && inClass {
			inClass = false

			continue
		}

		if char != '.' || inClass {
			continue
		}

		if i+1 < len(pattern) {
			next := pattern[i+1]
			if next == '*' || next == '+' || next == '?' {
				continue
			}
		}

		return true
	}

	return false
}

func (c *checker) warnMode(cfg model.LoadConfig) {
	if cfg.Mode == model.ModeIncremental && !cfg.HasPrimaryKey() {
		c.warn("mappings", "INCREMENTAL loads need a mapping with primaryKey set")
	}
}

func (c *checker) warnPlaceholders(cfg model.LoadConfig) {
	for name, mapping := range cfg.Mappings {
		if isPlaceholder(mapping.Src) {
			c.warn("mappings."+name+".src", "src looks like a placeholder expression")
		}
	}
}

func isPlaceholder(src string) bool {
	lowered := strings.ToLower(strings.TrimSpace(src))
	switch lowered {
	case "xxx", `"xxx"`, "'xxx'", "todo", "changeme":
		return true
	default:
		return strings.Contains(lowered, `"xxx"`) || strings.Contains(lowered, "'xxx'")
	}
}

func (c *checker) warnDelimiter(cfg model.LoadConfig) {
	if cfg.BQParams.FieldDelimiter == `\t` {
		c.warn("bqParams.fieldDelimiter", "delimiter is the two-character text \\t; store a real tab character")
	}
}

func (c *checker) warnColumnNames(cfg model.LoadConfig) {
	columnName := regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	for name := range cfg.Mappings {
		if !columnName.MatchString(name) {
			c.warn("mappings."+name, "column name is not a plain BigQuery identifier")
		}
	}
}

func (c *checker) warnClassificationMismatch(cfg model.LoadConfig) {
	fromID := importFromID(cfg.ID)

	fromTable := importFromTable(cfg.Destination.TableID)
	if fromID != "" && fromTable != "" && fromID != fromTable {
		c.warn("importType",
			"document id implies "+fromID+" but destination.tableId implies "+fromTable+
				"; the id wins, and the loader does not read importType")
	}
}

func importFromID(id string) string {
	_, importType := model.Classify(model.LoadConfig{ID: id})

	return importType
}

func importFromTable(tableID string) string {
	_, importType := model.Classify(model.LoadConfig{
		Destination: model.Destination{TableID: tableID},
	})

	return importType
}

func (c *checker) warnOverlaps(cfg model.LoadConfig, others []model.LoadConfig) {
	for _, other := range others {
		if other.ID == cfg.ID {
			continue
		}

		if patternsOverlap(cfg, other) {
			c.warn("patterns",
				"overlaps config "+other.ID+"; the loader keeps the first match in document id order")
		}
	}
}

func patternsOverlap(left, right model.LoadConfig) bool {
	leftPatterns := nonEmptyPatterns(left)

	rightPatterns := nonEmptyPatterns(right)
	for _, leftPattern := range leftPatterns {
		for _, rightPattern := range rightPatterns {
			if patternPairOverlaps(leftPattern, rightPattern) {
				return true
			}
		}
	}

	return false
}

func nonEmptyPatterns(cfg model.LoadConfig) []string {
	patterns := make([]string, 0, 2)
	if cfg.Patterns.Preprocess != "" {
		patterns = append(patterns, cfg.Patterns.Preprocess)
	}

	if cfg.Patterns.Ingest != "" {
		patterns = append(patterns, cfg.Patterns.Ingest)
	}

	return patterns
}

func patternPairOverlaps(left, right string) bool {
	if left == right {
		return true
	}

	leftRE, leftErr := compilePattern(left)

	rightRE, rightErr := compilePattern(right)
	if leftErr != nil || rightErr != nil {
		return false
	}

	if sample, ok := literalSample(left); ok && rightRE.MatchString(sample) {
		return true
	}

	if sample, ok := literalSample(right); ok && leftRE.MatchString(sample) {
		return true
	}

	return false
}

func literalSample(pattern string) (string, bool) {
	body := strings.TrimPrefix(pattern, "^")

	body = strings.TrimSuffix(body, "$")
	if body == "" {
		return "", false
	}

	var builder strings.Builder

	for i := 0; i < len(body); i++ {
		switch body[i] {
		case '\\':
			if i+1 >= len(body) {
				return "", false
			}

			builder.WriteByte(body[i+1])
			i++
		case '.':
			if i+1 < len(body) && (body[i+1] == '*' || body[i+1] == '+') {
				builder.WriteString("sample")

				i++

				continue
			}

			builder.WriteByte('x')
		case '[', '(', '|', '?', '*', '+', '{', '}':
			return "", false
		default:
			builder.WriteByte(body[i])
		}
	}

	sample := builder.String()

	compiled, err := compilePattern(pattern)
	if err != nil || !compiled.MatchString(sample) {
		return "", false
	}

	return sample, true
}
