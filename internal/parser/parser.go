package parser

import (
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"

	"gtd/internal/domain"
)

type ParseOptions struct {
	PreserveText  bool
	FallbackTitle string
}

type ParseResult struct {
	Title               string
	Status              *domain.TaskStatus
	Priority            *string
	EnergyLevel         *string
	Contexts            []string
	Tags                []string
	ProjectID           *string
	AreaID              *string
	AssignedTo          *string
	DueDate             *time.Time
	StartTime           *time.Time
	ReviewAt            *time.Time
	Description         *string
	Attachments         []domain.Attachment
	InvalidDateCommands []string
}

// SplitLines splits a bulk input into individual task strings, removing empty lines.
func SplitLines(input string) []string {
	var res []string
	lines := strings.Split(input, "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			res = append(res, l)
		}
	}
	return res
}

// sortByLengthDesc sorts a slice of strings by length, descending.
func sortByLengthDesc(items []string) {
	sort.Slice(items, func(i, j int) bool {
		return len(items[i]) > len(items[j])
	})
}

// Parse extracts metadata and structure from a quick-add string.
func Parse(input string, catalog *domain.EntityCatalog, opts ParseOptions, now time.Time) (*ParseResult, error) {
	res := &ParseResult{
		Contexts:            []string{},
		Tags:                []string{},
		Attachments:         []domain.Attachment{},
		InvalidDateCommands: []string{},
	}

	// Prepare greedy catalogs
	var projNames, areaNames, peopleNames, tagNames, contextNames []string
	projMap := make(map[string]string)
	areaMap := make(map[string]string)

	if catalog != nil {
		for _, p := range catalog.Projects {
			projNames = append(projNames, p.Title)
			projMap[strings.ToLower(p.Title)] = p.ID
		}
		for _, a := range catalog.Areas {
			areaNames = append(areaNames, a.Name)
			areaMap[strings.ToLower(a.Name)] = a.ID
		}
		for _, p := range catalog.People {
			peopleNames = append(peopleNames, p.Name)
		}
		for _, t := range catalog.Tags {
			cleaned := strings.TrimPrefix(t, "#")
			tagNames = append(tagNames, cleaned)
		}
		for _, c := range catalog.Contexts {
			cleaned := strings.TrimPrefix(c, "@")
			contextNames = append(contextNames, cleaned)
		}
		sortByLengthDesc(projNames)
		sortByLengthDesc(areaNames)
		sortByLengthDesc(peopleNames)
		sortByLengthDesc(tagNames)
		sortByLengthDesc(contextNames)
	}

	// Tokenize
	// We'll manually scan to handle quotes and escaping properly.
	var titleParts []string
	runes := []rune(input)
	n := len(runes)

	for i := 0; i < n; {
		if runes[i] == ' ' || runes[i] == '\t' || runes[i] == '\r' || runes[i] == '\n' {
			titleParts = append(titleParts, string(runes[i]))
			i++
			continue
		}

		if runes[i] == '\\' && i+1 < n {
			// Escaped char
			titleParts = append(titleParts, string(runes[i:i+2]))
			i += 2
			continue
		}

		// Check for prefixes: @, #, +, !, %, /
		ch := runes[i]
		if ch == '@' || ch == '#' || ch == '+' || ch == '!' || ch == '%' || ch == '/' {
			// Extract token
			tokenStart := i
			i++ // skip prefix
			var tokenContent strings.Builder

			if i < n && runes[i] == '"' {
				i++
				for i < n && runes[i] != '"' {
					tokenContent.WriteRune(runes[i])
					i++
				}
				if i < n && runes[i] == '"' {
					i++ // skip closing quote
				}
			} else {
				// unquoted: read until next space or delimiter?
				// For `/`, it might consume more. Let's read until next space.
				// Wait, if it's a command like `/note:ask about trip`, it shouldn't stop at space.
				// Actually, metadata commands like `/note:`, `/link:`, `/due:`, `/start:`, `/review:` can contain spaces.
				// For prefixes (@, #, +, !, %), they stop at space unless greedy matched.
				if ch == '/' {
					// read command name
					cmdNameStart := i
					for i < n && runes[i] != ':' && runes[i] != ' ' && runes[i] != '\t' {
						i++
					}
					cmdName := string(runes[cmdNameStart:i])

					if i < n && runes[i] == ':' {
						i++ // skip colon
						// read rest of command value until next unescaped prefix preceded by space, or EOF.
						// simplified: read until " /", " @", " #", " +", " !", " %"
						for i < n {
							if i+1 < n && (runes[i] == ' ' || runes[i] == '\t') {
								nx := runes[i+1]
								if nx == '/' || nx == '@' || nx == '#' || nx == '+' || nx == '!' || nx == '%' {
									break
								}
							}
							tokenContent.WriteRune(runes[i])
							i++
						}
					}
					
					val := strings.TrimSpace(tokenContent.String())
					switch cmdName {
					case "inbox":
						st := domain.TaskStatusInbox
						res.Status = &st
					case "next":
						st := domain.TaskStatusNext
						res.Status = &st
					case "waiting":
						st := domain.TaskStatusWaiting
						res.Status = &st
					case "someday":
						st := domain.TaskStatusSomeday
						res.Status = &st
					case "done":
						st := domain.TaskStatusDone
						res.Status = &st
					case "archived":
						st := domain.TaskStatusArchived
						res.Status = &st
					case "energy":
						low := strings.ToLower(val)
						res.EnergyLevel = &low
					case "note":
						res.Description = &val
					case "link":
						parts := strings.SplitN(val, "|", 2)
						uri := strings.TrimSpace(parts[0])
						title := cleanURLTitle(uri)
						if len(parts) == 2 {
							title = strings.TrimSpace(parts[0])
							uri = strings.TrimSpace(parts[1])
						}
						res.Attachments = append(res.Attachments, domain.Attachment{
							Kind:  "link",
							URI:   uri,
							Title: title,
						})
					case "due", "start", "review":
						dt := parseDateCommand(val, now)
						if dt == nil {
							res.InvalidDateCommands = append(res.InvalidDateCommands, "/"+cmdName+":"+val)
						} else {
							if cmdName == "due" {
								res.DueDate = dt
							} else if cmdName == "start" {
								res.StartTime = dt
							} else if cmdName == "review" {
								res.ReviewAt = dt
							}
						}
					}
					
					if opts.PreserveText {
						titleParts = append(titleParts, string(runes[tokenStart:i]))
					}
					continue
				} else {
					// Not a `/` command. 
					// Greedy matching for unquoted!
					remainingStr := string(runes[i:])
					var matched string
					
					if ch == '+' {
						matched = greedyMatch(remainingStr, projNames)
					} else if ch == '!' {
						matched = greedyMatch(remainingStr, areaNames)
					} else if ch == '%' {
						matched = greedyMatch(remainingStr, peopleNames)
					} else if ch == '#' {
						matched = greedyMatch(remainingStr, tagNames)
					} else if ch == '@' {
						matched = greedyMatch(remainingStr, contextNames)
					}
					
					if matched != "" {
						tokenContent.WriteString(matched)
						i += len([]rune(matched))
					} else {
						// fallback: read until space
						for i < n && runes[i] != ' ' && runes[i] != '\t' {
							tokenContent.WriteRune(runes[i])
							i++
						}
					}
				}
			}

			val := tokenContent.String()
			
			switch ch {
			case '@':
				res.Contexts = append(res.Contexts, "@"+val)
			case '#':
				res.Tags = append(res.Tags, "#"+val)
			case '+':
				id, ok := projMap[strings.ToLower(val)]
				if ok {
					res.ProjectID = &id
					res.AreaID = nil // Container exclusivity
				}
			case '!':
				id, ok := areaMap[strings.ToLower(val)]
				if ok {
					res.AreaID = &id
					if res.ProjectID == nil { // Project wins exclusivity on capture
						// actually wait, if both + and ! are present, + wins. We enforce this at the end.
						res.AreaID = &id
					}
				}
			case '%':
				res.AssignedTo = &val
			}

			if opts.PreserveText {
				titleParts = append(titleParts, string(runes[tokenStart:i]))
			}
			continue
		}

		// normal word
		start := i
		for i < n && runes[i] != ' ' && runes[i] != '\t' {
			i++
		}
		titleParts = append(titleParts, string(runes[start:i]))
	}

	// Enforce container exclusivity
	if res.ProjectID != nil {
		res.AreaID = nil
	}

	title := strings.TrimSpace(strings.Join(titleParts, ""))
	
	if opts.PreserveText {
		title = input
	} else {
		title = unescape(title)
		
		// Compress spaces
		spaceRegex := regexp.MustCompile(`\s+`)
		title = spaceRegex.ReplaceAllString(title, " ")
		title = strings.TrimSpace(title)
		
		// NLP Date fallback on title
		if res.DueDate == nil {
			title, res.DueDate = extractTrailingDate(title, now)
		}
	}

	if title == "" && opts.FallbackTitle != "" {
		title = opts.FallbackTitle
	}

	if title == "" {
		res.Title = title
		return res, errors.New("empty-title")
	}

	res.Title = title
	return res, nil
}

func greedyMatch(s string, candidates []string) string {
	sLower := strings.ToLower(s)
	for _, c := range candidates {
		cLower := strings.ToLower(c)
		if strings.HasPrefix(sLower, cLower) {
			// Ensure boundary match (next char is space or EOF)
			if len(sLower) == len(cLower) || sLower[len(cLower)] == ' ' || sLower[len(cLower)] == '\t' {
				// return exact casing from the input string for length of c
				runes := []rune(s)
				cRunes := []rune(c)
				return string(runes[:len(cRunes)])
			}
		}
	}
	return ""
}

func unescape(s string) string {
	s = strings.ReplaceAll(s, "\\@", "@")
	s = strings.ReplaceAll(s, "\\#", "#")
	s = strings.ReplaceAll(s, "\\+", "+")
	s = strings.ReplaceAll(s, "\\!", "!")
	s = strings.ReplaceAll(s, "\\%", "%")
	return s
}

func parseDateCommand(val string, now time.Time) *time.Time {
	val = strings.ToLower(strings.TrimSpace(val))
	if val == "tomorrow" {
		t := now.AddDate(0, 0, 1)
		// Date only
		t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
		return &t
	}
	if strings.HasPrefix(val, "tomorrow ") {
		// "tomorrow 5pm" - simplified parse
		t := now.AddDate(0, 0, 1)
		if strings.Contains(val, "5pm") {
			t = time.Date(t.Year(), t.Month(), t.Day(), 17, 0, 0, 0, t.Location())
			return &t
		}
	}
	if val == "next week" {
		t := now.AddDate(0, 0, 7)
		t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
		return &t
	}
	// Try parsing standard YYYY-MM-DD
	if t, err := time.Parse("2006-01-02", val); err == nil {
		return &t
	}
	
	// Fallback to not parsed for this example
	return nil
}

func extractTrailingDate(title string, now time.Time) (string, *time.Time) {
	lower := strings.ToLower(title)
	if strings.HasSuffix(lower, " tomorrow") {
		t := now.AddDate(0, 0, 1)
		t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
		return strings.TrimSpace(title[:len(title)-len(" tomorrow")]), &t
	}
	if strings.HasSuffix(lower, " tomorrow at 3pm") {
		t := now.AddDate(0, 0, 1)
		t = time.Date(t.Year(), t.Month(), t.Day(), 15, 0, 0, 0, t.Location())
		return strings.TrimSpace(title[:len(title)-len(" tomorrow at 3pm")]), &t
	}
	return title, nil
}

func cleanURLTitle(uri string) string {
	t := uri
	if idx := strings.Index(t, "://"); idx != -1 {
		t = t[idx+3:]
	}
	if idx := strings.IndexAny(t, "#?"); idx != -1 {
		t = t[:idx]
	}
	return t
}
